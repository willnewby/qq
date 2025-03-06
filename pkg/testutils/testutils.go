// Package testutils provides utilities for testing
package testutils

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// SetupTestDatabase starts a PostgreSQL container and returns a connection URL
func SetupTestDatabase(t *testing.T) (string, func()) {
	t.Helper()

	// Check if Docker is available
	dockerCmd := exec.Command("docker", "info")
	if err := dockerCmd.Run(); err != nil {
		t.Skip("Docker not available, skipping integration test")
	}

	// Generate a unique container name
	containerName := fmt.Sprintf("qq-test-db-%d", time.Now().UnixNano())
	
	// Make sure to remove any existing container with this name (just in case)
	cleanCmd := exec.Command("docker", "rm", "-f", containerName)
	_ = cleanCmd.Run() // Ignore errors if container doesn't exist
	
	// Start PostgreSQL container
	dockerArgs := []string{
		"run", "--rm", "-d",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_DB=qq_test",
		"-p", "5432", // Let Docker assign a random port
		"--name", containerName,
		"postgres:14-alpine",
	}
	
	startCmd := exec.Command("docker", dockerArgs...)
	output, err := startCmd.CombinedOutput()
	require.NoError(t, err, "Failed to start Docker container: %s", output)

	// Get the assigned port
	portCmd := exec.Command("docker", "port", containerName, "5432/tcp")
	portOutput, err := portCmd.CombinedOutput()
	require.NoError(t, err, "Failed to get container port: %s", portOutput)
	
	// Extract the port from output like "0.0.0.0:49153"
	port := ""
	_, err = fmt.Sscanf(string(portOutput), "0.0.0.0:%s", &port)
	require.NoError(t, err, "Failed to parse port from output: %s", portOutput)
	
	// Wait for PostgreSQL to be ready
	time.Sleep(3 * time.Second)
	
	// Connection URL
	dbURL := fmt.Sprintf("postgres://postgres:postgres@localhost:%s/qq_test", port)
	
	// Test the connection
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "Failed to connect to test database")
	pool.Close()
	
	// Return database URL and cleanup function
	cleanup := func() {
		stopCmd := exec.Command("docker", "stop", containerName)
		_ = stopCmd.Run()
	}
	
	return dbURL, cleanup
}