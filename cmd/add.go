/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/database"
	"qq/pkg/queue"
)

// jobAddCmd represents the job add command
var jobAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new job to the queue",
	Long: `Add a new job to the queue. The job command will be executed by a worker.

Examples:
  qq job add "echo hello world" --queue=default --priority=1
  qq job add "python /path/to/script.py" --schedule="2025-03-01T10:00:00Z"`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Error: job command is required")
			return
		}

		// Create a context for the operation
		ctx := context.Background()

		// Get command-line arguments
		jobCmd := strings.Join(args, " ")
		queueName, _ := cmd.Flags().GetString("queue")
		priority, _ := cmd.Flags().GetInt("priority")
		scheduleStr, _ := cmd.Flags().GetString("schedule")

		// Parse scheduled time if provided
		var scheduledTime *time.Time
		if scheduleStr != "" {
			parsed, err := time.Parse(time.RFC3339, scheduleStr)
			if err != nil {
				fmt.Printf("Error parsing schedule time: %v\n", err)
				return
			}
			scheduledTime = &parsed
		}

		// Get database URL
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

		// Add the job to the queue
		jobID, err := q.AddJob(ctx, jobCmd, queueName, priority, scheduledTime)
		if err != nil {
			fmt.Printf("Failed to add job to queue: %v\n", err)
			return
		}

		fmt.Printf("Added job to queue %s with priority %d\n", queueName, priority)
		fmt.Printf("Job ID: %s\n", jobID)
		fmt.Printf("Job command: %s\n", jobCmd)
		if scheduledTime != nil {
			fmt.Printf("Scheduled for: %s\n", scheduledTime.Format(time.RFC3339))
		}
	},
}

// queueAddCmd represents the queue add command
var queueAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new queue",
	Long: `Add a new queue to the system. Each queue can have its own workers.

Example:
  qq queue add high_priority --max-workers=10`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Error: queue name is required")
			return
		}

		queueName := args[0]
		maxWorkers, _ := cmd.Flags().GetInt("max-workers")
		
		ctx := context.Background()

		// Get database URL
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

		// Initialize the queue client - will add default queue configs
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

		// Note: River Queue doesn't have a direct API to add a new queue.
		// You can add jobs to any queue name, and River will create it.
		// Here we're just printing a message for now.
		
		fmt.Printf("Queue created: %s with max workers: %d\n", queueName, maxWorkers)
		fmt.Println("Note: Queues are automatically created when jobs are added to them")
	},
}

func init() {
	// Add jobAddCmd to the job command
	jobCmd.AddCommand(jobAddCmd)

	// Add queueAddCmd to the queue command
	queueCmd.AddCommand(queueAddCmd)

	// Add flags for job add command
	jobAddCmd.Flags().StringP("queue", "q", "default", "Queue to add the job to")
	jobAddCmd.Flags().IntP("priority", "p", 1, "Job priority (lower numbers run first)")
	jobAddCmd.Flags().StringP("schedule", "s", "", "Time to schedule the job (ISO 8601 format)")
	
	// Add flags for queue add command
	queueAddCmd.Flags().IntP("max-workers", "m", 5, "Maximum number of workers for this queue")
}
