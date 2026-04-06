/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/database"
	"qq/pkg/queue"
)

// applyCmd represents the apply command
var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Submit a pipeline of jobs from a YAML file",
	Long: `Submit a pipeline of jobs with dependencies from a YAML file.
All jobs are inserted atomically — either all succeed or none are created.

Example YAML file:
  jobs:
    - name: build
      command: "make build"
      queue: ci

    - name: test
      command: "make test"
      depends_on:
        - name: build
          condition: succeeded

Examples:
  qq apply -f pipeline.yaml
  qq apply -f pipeline.yaml --db-url=postgres://localhost:5432/mydb`,
	Run: func(cmd *cobra.Command, args []string) {
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			fmt.Println("Error: -f/--file flag is required")
			os.Exit(1)
		}

		// Check file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			fmt.Printf("Error: file not found: %s\n", filePath)
			os.Exit(1)
		}

		// Get database URL
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			fmt.Println("Database URL is required. Use --db-url flag or set it in the config file.")
			os.Exit(1)
		}

		ctx := context.Background()

		// Connect to the database
		db, err := database.New(ctx, dbURL)
		if err != nil {
			fmt.Printf("Failed to connect to the database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		// Create insert-only client (no workers started)
		q, err := queue.NewInsertOnlyClient(ctx, db.Pool)
		if err != nil {
			fmt.Printf("Failed to initialize queue client: %v\n", err)
			os.Exit(1)
		}

		// Apply the pipeline file
		results, err := q.ApplyFile(ctx, filePath)
		if err != nil {
			fmt.Printf("Failed to apply pipeline: %v\n", err)
			os.Exit(1)
		}

		// Print results table
		fmt.Printf("\n%-20s %-10s %-15s\n", "NAME", "JOB ID", "QUEUE")
		fmt.Printf("%-20s %-10s %-15s\n", "----", "------", "-----")
		depCount := 0
		for _, r := range results {
			fmt.Printf("%-20s %-10d %-15s\n", r.Name, r.JobID, r.Queue)
		}

		// Count dependencies from the file for summary
		af, _ := queue.ParseApplyFile(filePath)
		if af != nil {
			for _, job := range af.Jobs {
				depCount += len(job.DependsOn)
			}
		}

		fmt.Printf("\nSubmitted %d jobs with %d dependencies\n", len(results), depCount)
	},
}

func init() {
	rootCmd.AddCommand(applyCmd)
	applyCmd.Flags().StringP("file", "f", "", "Path to pipeline YAML file (required)")
}
