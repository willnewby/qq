package queue

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"qq/pkg/database"
)

// BashJobArgs defines a job that executes a bash command
type BashJobArgs struct {
	Command string `json:"command"`
}

// Kind returns the job kind
func (j BashJobArgs) Kind() string { return "bash_command" }

// BashWorker implements a worker for BashJobArgs
type BashWorker struct {
	pool         *pgxpool.Pool
	jobTableName string
	river.WorkerDefaults[BashJobArgs]
}

// checkDependencies checks if all dependencies for a job are satisfied.
// Returns (true, nil) if all deps are met, (false, nil) if some are pending,
// or (false, error) if a dep failed and condition is "succeeded" (returns JobCancel).
func (w *BashWorker) checkDependencies(ctx context.Context, jobID int64) (bool, error) {
	if w.jobTableName == "" {
		return true, nil
	}

	rows, err := w.pool.Query(ctx, fmt.Sprintf(`
		SELECT d.condition, j.state, r.exit_code
		FROM job_dependencies d
		JOIN %s j ON j.id = d.depends_on_job_id
		LEFT JOIN job_results r ON r.job_id = j.id AND r.attempt = j.attempt
		WHERE d.job_id = $1
	`, w.jobTableName), jobID)
	if err != nil {
		return false, fmt.Errorf("failed to query dependencies: %w", err)
	}
	defer rows.Close()

	hasDeps := false
	for rows.Next() {
		hasDeps = true
		var condition, state string
		var exitCode sql.NullInt32
		if err := rows.Scan(&condition, &state, &exitCode); err != nil {
			return false, fmt.Errorf("failed to scan dependency: %w", err)
		}

		switch {
		case state == "completed" && condition == "finished":
			// satisfied
		case state == "completed" && condition == "succeeded":
			if exitCode.Valid && exitCode.Int32 == 0 {
				// satisfied
			} else {
				return false, river.JobCancel(fmt.Errorf("dependency failed: exit code %d", exitCode.Int32))
			}
		case (state == "discarded" || state == "cancelled") && condition == "finished":
			// satisfied — terminal state counts for "finished"
		case state == "discarded" || state == "cancelled":
			// condition is "succeeded" but dep is in a failed terminal state
			return false, river.JobCancel(fmt.Errorf("dependency %s", state))
		default:
			// dependency not yet in terminal state
			return false, nil
		}
	}

	if !hasDeps {
		return true, nil
	}
	return true, nil
}

// Work executes the bash command
func (w *BashWorker) Work(ctx context.Context, job *river.Job[BashJobArgs]) error {
	// Check dependencies before executing
	satisfied, err := w.checkDependencies(ctx, job.ID)
	if err != nil {
		return err // JobCancel for failed deps
	}
	if !satisfied {
		return river.JobSnooze(5 * time.Second)
	}

	// Execute the command
	cmd := exec.CommandContext(ctx, "bash", "-c", job.Args.Command)
	output, cmdErr := cmd.CombinedOutput()

	// Extract exit code
	exitCode := 0
	if cmdErr != nil {
		fmt.Println("Job failed:", job.Args.Command)
		fmt.Println("Error:", cmdErr)
		fmt.Println("Output:", string(output))

		// Try to get the exit code from the error
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1 // Generic error code if we can't determine the actual code
		}
	} else {
		fmt.Println("Job completed successfully:", job.Args.Command)
		fmt.Println("Output:", string(output))
	}

	// Store the result in the database
	if w.pool != nil {
		saveErr := w.saveJobResult(ctx, job.ID, job.Attempt, string(output), exitCode)
		if saveErr != nil {
			fmt.Println("Failed to save job result:", saveErr)
		}
	}

	// Cancel the job so River marks it as discarded (failed) immediately
	// instead of retrying it
	if cmdErr != nil {
		return river.JobCancel(fmt.Errorf("command failed with exit code %d: %w", exitCode, cmdErr))
	}

	return nil
}

// saveJobResult stores the command output and exit code in the database
func (w *BashWorker) saveJobResult(ctx context.Context, jobID int64, attempt int, output string, exitCode int) error {
	_, err := w.pool.Exec(ctx, `
		INSERT INTO job_results (job_id, attempt, output, exit_code, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (job_id, attempt) DO UPDATE SET
			output = $3,
			exit_code = $4,
			created_at = NOW()
	`, jobID, attempt, output, exitCode)

	return err
}

// QueueStats represents statistics for a queue
type QueueStats struct {
	Name      string
	Pending   int
	Running   int
	Completed int
	Failed    int
}

// JobDependency represents a dependency between two jobs
type JobDependency struct {
	JobID       int64
	DependsOnID int64
	Condition   string
}

// QueueClient represents a client for interacting with River Queue
type QueueClient struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

// resolveJobTableName detects which table name River uses for jobs
func resolveJobTableName(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var name string
	err := pool.QueryRow(ctx, `
		SELECT table_name FROM information_schema.columns
		WHERE table_name IN ('river_job', 'river_queue_job')
		LIMIT 1
	`).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("failed to detect job table name: %w", err)
	}
	return name, nil
}

// WorkerConfig holds configuration for the River worker client.
type WorkerConfig struct {
	Concurrency int    // Max concurrent workers (default 5)
	Queue       string // Queue name to process (default "default")
}

// NewQueueClient creates a new client for interacting with River Queue.
// It starts worker goroutines that process jobs from the queue.
// Use NewInsertOnlyClient instead if you only need to read data or insert jobs.
func NewQueueClient(ctx context.Context, db interface{}, cfg *WorkerConfig) (*QueueClient, error) {
	pool, err := resolvePool(db)
	if err != nil {
		return nil, err
	}

	// Resolve job table name for the worker
	jobTableName, _ := resolveJobTableName(ctx, pool)

	// Create a River driver with the database pool
	driver := riverpgxv5.New(pool)

	// Create a new worker service with worker implementations
	workers := river.NewWorkers()
	river.AddWorker[BashJobArgs](workers, &BashWorker{
		pool:         pool,
		jobTableName: jobTableName,
	})

	// Apply defaults
	maxWorkers := 5
	queueName := "default"
	if cfg != nil {
		if cfg.Concurrency > 0 {
			maxWorkers = cfg.Concurrency
		}
		if cfg.Queue != "" {
			queueName = cfg.Queue
		}
	}

	// Create River client with the driver and workers
	riverConfig := &river.Config{
		Queues: map[string]river.QueueConfig{
			queueName: {MaxWorkers: maxWorkers},
		},
		Workers: workers,
	}

	client, err := river.NewClient(driver, riverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create river client: %w", err)
	}

	// Start River client
	if err := client.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start river client: %w", err)
	}

	return &QueueClient{
		client: client,
		pool:   pool,
	}, nil
}

// NewInsertOnlyClient creates a client that can insert jobs but does not start workers.
// Use this for commands like "qq apply" that only need to submit jobs.
func NewInsertOnlyClient(ctx context.Context, db interface{}) (*QueueClient, error) {
	pool, err := resolvePool(db)
	if err != nil {
		return nil, err
	}

	driver := riverpgxv5.New(pool)

	// Insert-only client: no workers, no queues
	riverConfig := &river.Config{}

	client, err := river.NewClient(driver, riverConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create river client: %w", err)
	}

	return &QueueClient{
		client: client,
		pool:   pool,
	}, nil
}

// resolvePool extracts a pgxpool.Pool from the db argument
func resolvePool(db interface{}) (*pgxpool.Pool, error) {
	switch v := db.(type) {
	case *pgxpool.Pool:
		return v, nil
	case *database.DB:
		return v.Pool, nil
	default:
		return nil, fmt.Errorf("unsupported database connection type: %T", db)
	}
}

// AddJob adds a new job to the queue
func (q *QueueClient) AddJob(ctx context.Context, cmd string, queueName string, priority int, scheduledTime *time.Time) (string, error) {
	// Create job args
	jobArgs := BashJobArgs{
		Command: cmd,
	}

	// Create insert options
	opts := &river.InsertOpts{}

	// Add queue name if specified
	if queueName != "" && queueName != "default" {
		opts.Queue = queueName
	}

	// Add priority if specified
	if priority > 0 {
		opts.Priority = priority
	}

	// Add scheduled time if specified
	if scheduledTime != nil {
		opts.ScheduledAt = *scheduledTime
	}

	// Insert the job into River Queue
	result, err := q.client.Insert(ctx, jobArgs, opts)
	if err != nil {
		return "", fmt.Errorf("failed to insert job: %w", err)
	}

	// Convert job ID to string
	return fmt.Sprintf("%d", result.Job.ID), nil
}

// AddDependency records a dependency between two jobs
func (q *QueueClient) AddDependency(ctx context.Context, jobID, dependsOnID int64, condition string) error {
	_, err := q.pool.Exec(ctx, `
		INSERT INTO job_dependencies (job_id, depends_on_job_id, condition)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, depends_on_job_id) DO NOTHING
	`, jobID, dependsOnID, condition)
	return err
}

// AddDependenciesTx records multiple dependencies within a transaction
func (q *QueueClient) AddDependenciesTx(ctx context.Context, tx pgx.Tx, deps []JobDependency) error {
	for _, dep := range deps {
		_, err := tx.Exec(ctx, `
			INSERT INTO job_dependencies (job_id, depends_on_job_id, condition)
			VALUES ($1, $2, $3)
		`, dep.JobID, dep.DependsOnID, dep.Condition)
		if err != nil {
			return fmt.Errorf("failed to insert dependency: %w", err)
		}
	}
	return nil
}

// GetDependencies retrieves all dependencies for a job
func (q *QueueClient) GetDependencies(ctx context.Context, jobID int64) ([]JobDependency, error) {
	rows, err := q.pool.Query(ctx, `
		SELECT job_id, depends_on_job_id, condition
		FROM job_dependencies
		WHERE job_id = $1
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []JobDependency
	for rows.Next() {
		var dep JobDependency
		if err := rows.Scan(&dep.JobID, &dep.DependsOnID, &dep.Condition); err != nil {
			return nil, err
		}
		deps = append(deps, dep)
	}
	return deps, nil
}

// RemoveJob cancels a job if it hasn't started yet
func (q *QueueClient) RemoveJob(ctx context.Context, jobID string) error {
	return fmt.Errorf("job cancellation not implemented in this version")
}

// JobInfo represents job information retrieved from the database
type JobInfo struct {
	ID          int64
	Queue       string
	State       string
	Command     string
	CreatedAt   time.Time
	ScheduledAt time.Time
	Output      string
	ExitCode    int
	Attempt     int
}

// ListJobs retrieves jobs from the database
func (q *QueueClient) ListJobs(ctx context.Context, queueName string, status string, limit int) ([]JobInfo, error) {
	jobTableName, err := resolveJobTableName(ctx, q.pool)
	if err != nil {
		return nil, err
	}

	// Build the SQL query
	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(fmt.Sprintf(`
		SELECT
			j.id,
			j.queue,
			j.state,
			j.args->>'command' as command,
			j.created_at,
			j.scheduled_at,
			j.attempt,
			r.output,
			r.exit_code
		FROM
			%s j
		LEFT JOIN
			job_results r ON j.id = r.job_id AND j.attempt = r.attempt
		WHERE 1=1
	`, jobTableName))

	// Add filters
	args := []interface{}{}
	argPos := 1

	if queueName != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND j.queue = $%d", argPos))
		args = append(args, queueName)
		argPos++
	}

	// Map our status to River's state
	if status != "" {
		switch status {
		case "pending":
			queryBuilder.WriteString(fmt.Sprintf(" AND j.state IN ('available', 'scheduled')"))
		case "running":
			queryBuilder.WriteString(fmt.Sprintf(" AND j.state = 'running'"))
		case "completed":
			queryBuilder.WriteString(fmt.Sprintf(" AND j.state = 'completed'"))
		case "failed":
			queryBuilder.WriteString(fmt.Sprintf(" AND j.state IN ('discarded', 'cancelled', 'retryable')"))
		}
	}

	// Add order by and limit
	queryBuilder.WriteString(" ORDER BY j.created_at DESC")
	queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", argPos))
	args = append(args, limit)

	// Execute the query
	pool := q.pool
	rows, err := pool.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	// Process the results
	var jobs []JobInfo
	for rows.Next() {
		var job JobInfo
		var command sql.NullString
		var output sql.NullString
		var exitCode sql.NullInt32
		var attempt sql.NullInt32

		if err := rows.Scan(
			&job.ID,
			&job.Queue,
			&job.State,
			&command,
			&job.CreatedAt,
			&job.ScheduledAt,
			&attempt,
			&output,
			&exitCode,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}

		if command.Valid {
			job.Command = command.String
		} else {
			job.Command = "Unknown command"
		}

		if attempt.Valid {
			job.Attempt = int(attempt.Int32)
		}

		if output.Valid {
			job.Output = output.String
		}

		if exitCode.Valid {
			job.ExitCode = int(exitCode.Int32)
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJob retrieves a single job by ID
func (q *QueueClient) GetJob(ctx context.Context, jobID int64) (*JobInfo, error) {
	jobTableName, err := resolveJobTableName(ctx, q.pool)
	if err != nil {
		return nil, err
	}

	var job JobInfo
	var command sql.NullString
	var output sql.NullString
	var exitCode sql.NullInt32
	var attempt sql.NullInt32

	err = q.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			j.id,
			j.queue,
			j.state,
			j.args->>'command' as command,
			j.created_at,
			j.scheduled_at,
			j.attempt,
			r.output,
			r.exit_code
		FROM
			%s j
		LEFT JOIN
			job_results r ON j.id = r.job_id AND j.attempt = r.attempt
		WHERE
			j.id = $1
	`, jobTableName), jobID).Scan(
		&job.ID,
		&job.Queue,
		&job.State,
		&command,
		&job.CreatedAt,
		&job.ScheduledAt,
		&attempt,
		&output,
		&exitCode,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if command.Valid {
		job.Command = command.String
	}
	if attempt.Valid {
		job.Attempt = int(attempt.Int32)
	}
	if output.Valid {
		job.Output = output.String
	}
	if exitCode.Valid {
		job.ExitCode = int(exitCode.Int32)
	}

	return &job, nil
}

// GetJobOutput retrieves the full output for a specific job
func (q *QueueClient) GetJobOutput(ctx context.Context, jobID int64) (string, int, error) {
	jobTableName, err := resolveJobTableName(ctx, q.pool)
	if err != nil {
		return "", 0, err
	}

	// Query the job_results table for this job
	var output sql.NullString
	var exitCode sql.NullInt32

	err = q.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			r.output,
			r.exit_code
		FROM
			%s j
		LEFT JOIN
			job_results r ON j.id = r.job_id AND j.attempt = r.attempt
		WHERE
			j.id = $1
	`, jobTableName), jobID).Scan(&output, &exitCode)

	if err != nil {
		return "", 0, fmt.Errorf("failed to get job output: %w", err)
	}

	// Convert nullable values to regular values
	outputStr := ""
	if output.Valid {
		outputStr = output.String
	}

	exitCodeInt := 0
	if exitCode.Valid {
		exitCodeInt = int(exitCode.Int32)
	}

	return outputStr, exitCodeInt, nil
}

// GetQueueStats retrieves statistics for all queues or a specific queue
func (q *QueueClient) GetQueueStats(ctx context.Context, queueName string) ([]QueueStats, error) {
	jobTableName, err := resolveJobTableName(ctx, q.pool)
	if err != nil {
		return nil, err
	}

	// Build query
	query := fmt.Sprintf(`
		SELECT
			queue,
			SUM(CASE WHEN state IN ('available', 'scheduled') THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN state = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN state = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN state IN ('discarded', 'cancelled', 'retryable') THEN 1 ELSE 0 END) as failed
		FROM
			%s
	`, jobTableName)

	args := []interface{}{}
	if queueName != "" {
		query += " WHERE queue = $1"
		args = append(args, queueName)
	}

	query += " GROUP BY queue"

	// Execute query
	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query queue stats: %w", err)
	}
	defer rows.Close()

	// Process results
	var stats []QueueStats
	for rows.Next() {
		var stat QueueStats
		if err := rows.Scan(&stat.Name, &stat.Pending, &stat.Running, &stat.Completed, &stat.Failed); err != nil {
			return nil, fmt.Errorf("failed to scan queue stats: %w", err)
		}
		stats = append(stats, stat)
	}

	return stats, nil
}

// WorkerInfo represents an active queue with worker activity.
// River tracks worker presence via the river_queue table, so each entry
// corresponds to a queue that has had recent worker activity.
type WorkerInfo struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
	Queues    []WorkerQueueInfo
}

// WorkerQueueInfo represents a queue that a worker is watching
type WorkerQueueInfo struct {
	Name             string
	MaxWorkers       int
	NumJobsRunning   int
	NumJobsCompleted int
}

// ListWorkers retrieves active workers by querying River's river_queue table.
// River producers periodically update river_queue.updated_at as a heartbeat
// (default interval is 10 minutes). We also count running jobs per queue from
// the river_job table to show current activity.
func (q *QueueClient) ListWorkers(ctx context.Context) ([]WorkerInfo, error) {
	// Resolve the job table name (river_job or schema-prefixed variant)
	jobTableName, _ := resolveJobTableName(ctx, q.pool)
	if jobTableName == "" {
		jobTableName = "river_job"
	}

	// Query active queues from river_queue. A queue is considered active if
	// updated within the last 11 minutes (River's default report interval is 10m).
	query := fmt.Sprintf(`
		SELECT
			rq.name,
			rq.created_at,
			rq.updated_at,
			COALESCE(running.cnt, 0) AS num_jobs_running
		FROM river_queue rq
		LEFT JOIN (
			SELECT queue, COUNT(*) AS cnt
			FROM %s
			WHERE state = 'running'
			GROUP BY queue
		) running ON running.queue = rq.name
		WHERE rq.updated_at > NOW() - INTERVAL '11 minutes'
		ORDER BY rq.name
	`, jobTableName)

	rows, err := q.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query workers: %w", err)
	}
	defer rows.Close()

	var workers []WorkerInfo
	for rows.Next() {
		var name string
		var createdAt, updatedAt time.Time
		var numRunning int

		if err := rows.Scan(&name, &createdAt, &updatedAt, &numRunning); err != nil {
			return nil, fmt.Errorf("failed to scan worker row: %w", err)
		}

		workers = append(workers, WorkerInfo{
			ID:        name,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			Queues: []WorkerQueueInfo{
				{
					Name:           name,
					NumJobsRunning: numRunning,
				},
			},
		})
	}

	return workers, nil
}

// Close stops the River client
func (q *QueueClient) Close(ctx context.Context) error {
	return q.client.Stop(ctx)
}

// Pool returns the underlying database pool for transaction use
func (q *QueueClient) Pool() *pgxpool.Pool {
	return q.pool
}

// Client returns the underlying River client for transaction-based inserts
func (q *QueueClient) Client() *river.Client[pgx.Tx] {
	return q.client
}
