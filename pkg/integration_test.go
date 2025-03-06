// Integration tests for the entire qq application
package pkg

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"qq/pkg/testutils"
)

// TestEndToEndIntegration runs a full end-to-end test with a temporary database
func TestEndToEndIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database using the shared test helper
	dbURL, cleanup := testutils.SetupTestDatabase(t)
	defer cleanup()

	// Clean build
	cmd := exec.Command("go", "build", "-o", "qq_test", ".")
	cmd.Dir = "../" // Run from project root
	err := cmd.Run()
	require.NoError(t, err, "Failed to build binary")
	defer os.Remove("../qq_test")

	// Initialize database
	initCmd := exec.Command("../qq_test", "init", "--db-url", dbURL)
	output, err := initCmd.CombinedOutput()
	require.NoError(t, err, "Failed to initialize database: %s", output)

	// Start worker in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCmd := exec.Command("../qq_test", "worker", "--db-url", dbURL, "--concurrency", "1")
	err = workerCmd.Start()
	require.NoError(t, err, "Failed to start worker")

	// Ensure worker is killed when test exits
	defer func() {
		_ = workerCmd.Process.Kill()
		_ = workerCmd.Wait()
	}()

	// Add a test job
	addCmd := exec.Command("../qq_test", "job", "add", "echo hello world", "--db-url", dbURL)
	output, err = addCmd.CombinedOutput()
	require.NoError(t, err, "Failed to add job: %s", output)
	assert.Contains(t, string(output), "Added job")

	// Wait for job to complete
	time.Sleep(3 * time.Second)

	// List jobs to verify
	lsCmd := exec.Command("../qq_test", "job", "ls", "--db-url", dbURL)
	output, err = lsCmd.CombinedOutput()
	require.NoError(t, err, "Failed to list jobs: %s", output)
	assert.Contains(t, string(output), "echo hello world")

	// Verify database directly
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	// Verify job_results table has an entry
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM job_results").Scan(&count)
	require.NoError(t, err, "Failed to query job_results")
	assert.GreaterOrEqual(t, count, 1, "Expected at least one job result")
}