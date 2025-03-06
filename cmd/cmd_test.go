package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	err = root.Execute()
	return buf.String(), err
}

func TestRootCommand(t *testing.T) {
	// Store original args and restore them after test
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Use the exported root command
	cmd := rootCmd
	output, err := executeCommand(cmd)

	assert.NoError(t, err)
	assert.Contains(t, output, "Usage:")
}

func TestJobCommandsHelp(t *testing.T) {
	// Test job help output
	cmd := rootCmd
	
	// Test job help
	output, err := executeCommand(cmd, "job", "--help")
	assert.NoError(t, err)
	// Just verify the command runs without error
	assert.Contains(t, output, "Usage:")
}

func TestQueueCommandsHelp(t *testing.T) {
	// Test queue help output
	cmd := rootCmd
	
	// Test queue help
	output, err := executeCommand(cmd, "queue", "--help")
	assert.NoError(t, err)
	// Just verify the command runs without error
	assert.Contains(t, output, "Usage:")
}