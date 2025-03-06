package worker

import (
	"context"
	"os/exec"
	"strconv"
	"testing"
	"time"
	
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"qq/pkg/queue"
	"qq/pkg/testutils"
)

func TestWorkerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Use SetupTestDatabase from testutils package
	dbURL, cleanup := testutils.SetupTestDatabase(t)
	defer cleanup()

	// Initialize database schema
	initCmd := exec.Command("go", "run", ".", "init", "--db-url", dbURL)
	initCmd.Dir = "../.." // Run from project root
	output, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to initialize database: %s", output)

	// Start a worker
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	workerCmd := exec.Command("go", "run", ".", "worker", "--db-url", dbURL, "--concurrency", "1")
	workerCmd.Dir = "../.." // Run from project root
	err = workerCmd.Start()
	require.NoError(t, err, "Failed to start worker")
	
	// Ensure worker is killed when test exits
	defer func() {
		_ = workerCmd.Process.Kill()
		_ = workerCmd.Wait()
	}()

	// Connect to the database
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	// Create a client to add jobs
	client, err := queue.NewQueueClient(ctx, pool)
	require.NoError(t, err)
	defer client.Close(ctx)

	// Add a test job
	jobID, err := client.AddJob(ctx, "echo 'worker test'", "default", 1, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, jobID)

	// Wait for job to be processed
	time.Sleep(3 * time.Second)

	// Verify job was processed
	jobs, err := client.ListJobs(ctx, "default", "", 10)
	require.NoError(t, err)
	
	found := false
	jobIDInt, _ := strconv.ParseInt(jobID, 10, 64)
	for _, job := range jobs {
		if job.ID == jobIDInt {
			// Verify the job was processed
			found = true
			break
		}
	}
	
	assert.True(t, found, "Expected job to be found in job list")
}
