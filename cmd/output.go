/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/database"
	"qq/pkg/queue"
)

// jobOutputCmd represents the job output command
var jobOutputCmd = &cobra.Command{
	Use:   "output [jobID]",
	Short: "Show the complete output of a job",
	Long: `Show the complete output of a job with the given ID.
This includes the full command output and exit code.

Example:
  qq job output 123`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Parse the job ID from the arguments
		jobID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Printf("Invalid job ID: %v\n", err)
			return
		}

		// Create a context for the operation
		ctx := context.Background()

		// Get database URL from config
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			fmt.Println("Database URL is required. Use --db-url flag or set it in the config file.")
			return
		}

		// Connect to the database
		db, err := database.New(ctx, dbURL)
		if err != nil {
			fmt.Printf("Failed to connect to the database: %v\n", err)
			return
		}
		defer db.Close()

		// Initialize the queue client
		q, err := queue.NewQueueClient(ctx, db.Pool)
		if err != nil {
			fmt.Printf("Failed to initialize the queue: %v\n", err)
			return
		}
		defer func() {
			if err := q.Close(context.Background()); err != nil {
				fmt.Printf("Failed to close the queue: %v\n", err)
			}
		}()

		// Get the job output
		output, exitCode, err := q.GetJobOutput(ctx, jobID)
		if err != nil {
			fmt.Printf("Failed to get job output: %v\n", err)
			return
		}

		// Display the job information
		fmt.Printf("Output for job %d:\n\n", jobID)
		fmt.Printf("Exit Code: %d\n\n", exitCode)
		fmt.Printf("Output:\n%s\n", output)
	},
}

func init() {
	jobCmd.AddCommand(jobOutputCmd)
}
