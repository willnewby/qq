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
	pool *pgxpool.Pool
	river.WorkerDefaults[BashJobArgs]
}

// Work executes the bash command
func (w *BashWorker) Work(ctx context.Context, job *river.Job[BashJobArgs]) error {
	// Execute the command
	cmd := exec.CommandContext(ctx, "bash", "-c", job.Args.Command)
	output, err := cmd.CombinedOutput()

	// Extract exit code
	exitCode := 0
	if err != nil {
		fmt.Println("Job failed:", job.Args.Command)
		fmt.Println("Error:", err)
		fmt.Println("Output:", string(output))

		// Try to get the exit code from the error
		if exitErr, ok := err.(*exec.ExitError); ok {
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

	// Return the original error if there was one
	if err != nil {
		return fmt.Errorf("command failed with exit code %d: %w", exitCode, err)
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

// QueueClient represents a client for interacting with River Queue
type QueueClient struct {
	client *river.Client[pgx.Tx]
	pool   *pgxpool.Pool
}

// NewQueueClient creates a new client for interacting with River Queue
func NewQueueClient(ctx context.Context, db interface{}) (*QueueClient, error) {
	var pool *pgxpool.Pool

	// Try to determine the connection pool
	switch v := db.(type) {
	case *pgxpool.Pool:
		pool = v
	case *database.DB:
		pool = v.Pool
	default:
		return nil, fmt.Errorf("unsupported database connection type: %T", db)
	}

	// Create a River driver with the database pool
	driver := riverpgxv5.New(pool)

	// Create a new worker service with worker implementations
	workers := river.NewWorkers()
	river.AddWorker[BashJobArgs](workers, &BashWorker{
		pool: pool,
	})

	// Create River client with the driver and workers
	riverConfig := &river.Config{
		Queues: map[string]river.QueueConfig{
			"default": {MaxWorkers: 5},
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

// RemoveJob cancels a job if it hasn't started yet
func (q *QueueClient) RemoveJob(ctx context.Context, jobID string) error {
	// With the current River API, there isn't a direct way to cancel jobs
	// by ID in a client-only operation without more complex queries
	// For now, we'll return an error indicating this isn't implemented
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
	// First, detect which table name is used by River
	var jobTableName string
	err := q.pool.QueryRow(ctx, `
		SELECT table_name FROM information_schema.columns
		WHERE table_name IN ('river_job', 'river_queue_job')
		LIMIT 1
	`).Scan(&jobTableName)
	if err != nil {
		return nil, fmt.Errorf("failed to detect job table name: %w", err)
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

// GetJobOutput retrieves the full output for a specific job
func (q *QueueClient) GetJobOutput(ctx context.Context, jobID int64) (string, int, error) {
	// First, detect which table name is used by River
	var jobTableName string
	err := q.pool.QueryRow(ctx, `
		SELECT table_name FROM information_schema.columns
		WHERE table_name IN ('river_job', 'river_queue_job')
		LIMIT 1
	`).Scan(&jobTableName)
	if err != nil {
		return "", 0, fmt.Errorf("failed to detect job table name: %w", err)
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
	// First, detect which table name is used by River
	var jobTableName string
	err := q.pool.QueryRow(ctx, `
		SELECT table_name FROM information_schema.columns
		WHERE table_name IN ('river_job', 'river_queue_job')
		LIMIT 1
	`).Scan(&jobTableName)
	if err != nil {
		return nil, fmt.Errorf("failed to detect job table name: %w", err)
	}

	// Build query
	query := fmt.Sprintf(`
		SELECT 
			queue, 
			SUM(CASE WHEN state IN ('available', 'scheduled', 'retryable') THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN state = 'running' THEN 1 ELSE 0 END) as running,
			SUM(CASE WHEN state = 'completed' THEN 1 ELSE 0 END) as completed,
			SUM(CASE WHEN state IN ('discarded', 'cancelled') THEN 1 ELSE 0 END) as failed
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

// Close stops the River client
func (q *QueueClient) Close(ctx context.Context) error {
	return q.client.Stop(ctx)
}