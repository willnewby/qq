package queue

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// BashJobArgs defines a job that executes a bash command
type BashJobArgs struct {
	Command string `json:"command"`
}

// Kind returns the job kind
func (j BashJobArgs) Kind() string { return "bash_command" }

// BashWorker implements a worker for BashJobArgs
type BashWorker struct {
	river.WorkerDefaults[BashJobArgs]
}

// Work executes the bash command
func (w *BashWorker) Work(ctx context.Context, job *river.Job[BashJobArgs]) error {
	// Execute the command
	cmd := exec.CommandContext(ctx, "bash", "-c", job.Args.Command)
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Println("Job failed:", job.Args.Command)
		fmt.Println("Error:", err)
		fmt.Println("Output:", string(output))
		return fmt.Errorf("command failed: %w", err)
	}

	fmt.Println("Job completed successfully:", job.Args.Command)
	fmt.Println("Output:", string(output))
	return nil
}

// QueueClient represents a client for interacting with River Queue
type QueueClient struct {
	client *river.Client[pgx.Tx]
}

// NewQueueClient creates a new client for interacting with River Queue
func NewQueueClient(ctx context.Context, pool *pgxpool.Pool) (*QueueClient, error) {
	// Create a River driver with the database pool
	driver := riverpgxv5.New(pool)

	// Create a new worker service with worker implementations
	workers := river.NewWorkers()
	river.AddWorker(workers, &BashWorker{})

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

// Close stops the River client
func (q *QueueClient) Close(ctx context.Context) error {
	return q.client.Stop(ctx)
}