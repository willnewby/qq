/*
Copyright © 2025 Will Atlas <will@atls.dev>
*/
package cmd

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/config"
	"qq/pkg/database"
	"qq/pkg/queue"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Starts a server that shows queue status",
	Long: `The server command starts a web server that shows the status
of the queue, including active, pending, and completed jobs.

The server connects to the same Postgres database as the workers
to provide real-time information about the queue status.`,
	Run: func(cmd *cobra.Command, args []string) {
		addr, _ := cmd.Flags().GetString("addr")
		fmt.Printf("Starting server on %s...\n", addr)

		// Create a context that's canceled when SIGINT or SIGTERM is received
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Set up a channel to listen for OS signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\nShutting down server...")
			cancel()
		}()

		// Load configuration
		_, err := config.LoadConfig()
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

		// Connect to the database
		fmt.Println("Connecting to the database...")
		db, err := database.New(ctx, dbURL)
		if err != nil {
			fmt.Printf("Failed to connect to the database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		// Set up HTTP routes
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// Simple HTML template for the dashboard
			tmpl := `
			<!DOCTYPE html>
			<html>
			<head>
				<title>QQ - Queue Dashboard</title>
				<style>
					body { font-family: Arial, sans-serif; margin: 0; padding: 20px; }
					h1 { color: #333; }
					table { border-collapse: collapse; width: 100%; }
					th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
					th { background-color: #f2f2f2; }
					tr:nth-child(even) { background-color: #f9f9f9; }
					.refresh { margin-bottom: 20px; }
				</style>
			</head>
			<body>
				<h1>QQ - Queue Dashboard</h1>
				<p class="refresh"><a href="/">Refresh</a></p>
				
				<h2>Queues</h2>
				<table>
					<tr>
						<th>Name</th>
						<th>Pending</th>
						<th>Running</th>
						<th>Completed</th>
						<th>Failed</th>
					</tr>
					{{range .Queues}}
					<tr>
						<td>{{.Name}}</td>
						<td>{{.Pending}}</td>
						<td>{{.Running}}</td>
						<td>{{.Completed}}</td>
						<td>{{.Failed}}</td>
					</tr>
					{{end}}
				</table>
				
				<h2>Recent Jobs</h2>
				<table>
					<tr>
						<th>ID</th>
						<th>Queue</th>
						<th>Command</th>
						<th>Status</th>
						<th>Created</th>
					</tr>
					{{range .Jobs}}
					<tr>
						<td>{{.ID}}</td>
						<td>{{.Queue}}</td>
						<td>{{.Command}}</td>
						<td>{{.Status}}</td>
						<td>{{.Created}}</td>
					</tr>
					{{end}}
				</table>
			</body>
			</html>
			`

			// Create queue client to get real data
			queueClient, err := queue.NewQueueClient(ctx, db)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create queue client: %v", err), http.StatusInternalServerError)
				return
			}
			defer queueClient.Close(ctx)

			// Get queue stats
			queueStats, err := queueClient.GetQueueStats(ctx, "")
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get queue stats: %v", err), http.StatusInternalServerError)
				return
			}

			// If no queues are found, add a default one with zeros
			if len(queueStats) == 0 {
				queueStats = append(queueStats, queue.QueueStats{
					Name:      "default",
					Pending:   0,
					Running:   0,
					Completed: 0,
					Failed:    0,
				})
			}

			// Get recent jobs
			jobs, err := queueClient.ListJobs(ctx, "", "", 100) // Get 10 most recent jobs
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to list jobs: %v", err), http.StatusInternalServerError)
				return
			}

			// Prepare data for template
			type templateJob struct {
				ID      string
				Queue   string
				Command string
				Status  string
				Created string
			}

			var templateJobs []templateJob
			for _, job := range jobs {
				status := "unknown"
				switch job.State {
				case "available", "scheduled", "retryable":
					status = "pending"
				case "running":
					status = "running"
				case "completed":
					status = "completed"
				case "discarded", "cancelled":
					status = "failed"
				}

				templateJobs = append(templateJobs, templateJob{
					ID:      fmt.Sprintf("%d", job.ID),
					Queue:   job.Queue,
					Command: job.Command,
					Status:  status,
					Created: job.CreatedAt.Format(time.RFC3339),
				})
			}

			data := struct {
				Queues []queue.QueueStats
				Jobs   []templateJob
			}{
				Queues: queueStats,
				Jobs:   templateJobs,
			}

			t, err := template.New("dashboard").Parse(tmpl)
			if err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
				return
			}

			if err := t.Execute(w, data); err != nil {
				http.Error(w, "Template execution error", http.StatusInternalServerError)
				return
			}
		})

		// Start HTTP server
		server := &http.Server{
			Addr: addr,
		}

		// Start the server in a goroutine
		go func() {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Printf("HTTP server error: %v\n", err)
			}
		}()

		fmt.Printf("Server is running on http://%s\n", addr)

		// Wait for context cancellation (from signal)
		<-ctx.Done()

		// Shutdown gracefully
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Server shutdown error: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	// Add flags specific to the server command
	serverCmd.Flags().StringP("addr", "a", ":8080", "Address to listen on (host:port)")
}
