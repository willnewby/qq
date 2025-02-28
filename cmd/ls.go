/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/database"
	"qq/pkg/queue"
)

// jobLsCmd represents the job ls command
var jobLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List jobs in the queue",
	Long: `List all jobs in the queue, optionally filtered by status.

Examples:
  qq job ls
  qq job ls --status=pending
  qq job ls --queue=high_priority`,
	Run: func(cmd *cobra.Command, args []string) {
		status, _ := cmd.Flags().GetString("status")
		queueName, _ := cmd.Flags().GetString("queue")
		limit, _ := cmd.Flags().GetInt("limit")

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

		// Fetch jobs from the database
		jobs, err := q.ListJobs(ctx, queueName, status, limit)
		if err != nil {
			fmt.Printf("Failed to list jobs: %v\n", err)
			return
		}

		fmt.Printf("Listing jobs (limit: %d)\n", limit)
		if status != "" {
			fmt.Printf("Filtered by status: %s\n", status)
		}
		if queueName != "" {
			fmt.Printf("Filtered by queue: %s\n", queueName)
		}

		fmt.Println("\nID\tQueue\t\tStatus    \tCommand\tExitCode")
		fmt.Println("------------------------------------------------------------------")

		if len(jobs) == 0 {
			fmt.Println("No jobs found.")
			return
		}

		// Display jobs
		for _, job := range jobs {
			// Map River state to our job status
			var status string
			switch job.State {
			case "available", "scheduled":
				status = "pending"
			case "running":
				status = "running"
			case "completed":
				status = "completed"
			case "discarded", "cancelled", "retryable":
				status = "failed    "
			default:
				status = job.State
			}

			fmt.Printf("%d\t%s\t\t%s\t%s\t%d\n", job.ID, job.Queue, status, job.Command, job.ExitCode)

		}
	},
}

// queueLsCmd represents the queue ls command
var queueLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all queues",
	Long: `List all queues in the system along with their statistics.

Example:
  qq queue ls`,
	Run: func(cmd *cobra.Command, args []string) {
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

		// Execute a query to get queue stats
		// River doesn't have a direct API for this, so we need to query the database
		rows, err := db.Pool.Query(ctx, `
			SELECT 
				queue, 
				COUNT(*) FILTER (WHERE state IN ('available', 'scheduled')) AS pending,
				COUNT(*) FILTER (WHERE state = 'running') AS running,
				COUNT(*) FILTER (WHERE state = 'completed') AS completed
			FROM 
				river_job
			GROUP BY 
				queue
			ORDER BY 
				queue
		`)
		if err != nil {
			fmt.Printf("Failed to query queues: %v\n", err)
			return
		}
		defer rows.Close()

		fmt.Println("Listing all queues:")
		fmt.Println("\nName\t\tPending\tRunning\tCompleted")
		fmt.Println("--------------------------------------------------")

		queueCount := 0
		for rows.Next() {
			var queueName string
			var pending, running, completed int

			if err := rows.Scan(&queueName, &pending, &running, &completed); err != nil {
				fmt.Printf("Error scanning row: %v\n", err)
				continue
			}

			fmt.Printf("%s\t\t%d\t%d\t%d\n", queueName, pending, running, completed)
			queueCount++
		}

		if queueCount == 0 {
			fmt.Println("No queues found.")
		}
	},
}

func init() {
	// Add jobLsCmd to the job command
	jobCmd.AddCommand(jobLsCmd)

	// Add queueLsCmd to the queue command
	queueCmd.AddCommand(queueLsCmd)

	// Add flags for job ls command
	jobLsCmd.Flags().StringP("status", "s", "", "Filter by status (pending, running, completed, failed)")
	jobLsCmd.Flags().StringP("queue", "q", "", "Filter by queue name")
	jobLsCmd.Flags().IntP("limit", "l", 20, "Limit the number of results")
}
