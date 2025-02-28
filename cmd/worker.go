/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/config"
	"qq/pkg/database"
	"qq/pkg/queue"
)

// workerCmd represents the worker command
var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Starts a worker that processes jobs from the queue",
	Long: `The worker command starts a process that listens for jobs
on the queue and executes them.

Workers connect to the Postgres database specified in the configuration
and process jobs based on their priority and scheduled time.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting worker...")

		// Create a context that's canceled when SIGINT or SIGTERM is received
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set up a channel to listen for OS signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\nShutting down worker...")
			cancel()
		}()

		// Load configuration
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Printf("Failed to load configuration: %v\n", err)
			os.Exit(1)
		}

		// Get database URL
		dbURL := viper.GetString("db_url")
		if dbURL == "" {
			fmt.Println("Database URL is required. Use --db-url flag or set it in the config file.")
			os.Exit(1)
		}

		// Update configuration with command-line flags
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		if concurrency > 0 {
			cfg.Worker.Concurrency = concurrency
		}

		queueName, _ := cmd.Flags().GetString("queue")
		if queueName != "" {
			cfg.Worker.Queue = queueName
		}

		interval, _ := cmd.Flags().GetDuration("interval")
		if interval > 0 {
			cfg.Worker.Interval = int(interval.Seconds())
		}

		// Connect to the database
		fmt.Println("Connecting to the database...")
		db, err := database.New(ctx, dbURL)
		if err != nil {
			fmt.Printf("Failed to connect to the database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		// Initialize the queue client
		fmt.Println("Initializing the queue...")
		q, err := queue.NewQueueClient(ctx, db.Pool)
		if err != nil {
			fmt.Printf("Failed to initialize the queue: %v\n", err)
			os.Exit(1)
		}
		defer func() {
			if err := q.Close(context.Background()); err != nil {
				fmt.Printf("Failed to close the queue: %v\n", err)
			}
		}()

		fmt.Printf("Worker is running with concurrency %d\n", concurrency)
		if queueName != "" {
			fmt.Printf("Processing queue: %s\n", queueName)
		} else {
			fmt.Println("Processing all queues")
		}

		// River Queue manages workers internally, so we just need to wait
		// for the context to be canceled
		<-ctx.Done()
	},
}

func init() {
	rootCmd.AddCommand(workerCmd)

	// Add flags specific to the worker command
	workerCmd.Flags().IntP("concurrency", "c", 1, "Number of jobs to process concurrently")
	workerCmd.Flags().StringP("queue", "q", "", "Queue to process (default: all queues)")
	workerCmd.Flags().DurationP("interval", "i", 0, "Polling interval for checking new jobs")
}
