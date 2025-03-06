package queue

import (
	"context"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"qq/pkg/testutils"
)

func TestQueueIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup a PostgreSQL database for testing
	dbURL, cleanup := testutils.SetupTestDatabase(t)
	defer cleanup()

	// Initialize database schema
	initCmd := exec.Command("go", "run", ".", "init", "--db-url", dbURL)
	initCmd.Dir = "../.." // Run from project root
	output, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to initialize database: %s", output)

	// Connect to the database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	// Initialize queue client
	q, err := NewQueueClient(ctx, pool)
	require.NoError(t, err)
	defer q.Close(ctx)

	// Add a job
	jobID, err := q.AddJob(ctx, "echo 'test'", "default", 1, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, jobID)

	// Verify job exists
	jobs, err := q.ListJobs(ctx, "default", "", 10)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(jobs), 1)
}