/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
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
					</tr>
					<tr>
						<td>default</td>
						<td>{{.DefaultPending}}</td>
						<td>{{.DefaultRunning}}</td>
						<td>{{.DefaultCompleted}}</td>
					</tr>
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

			// Sample data - in a real implementation, this would query River Queue
			data := struct {
				DefaultPending   int
				DefaultRunning   int
				DefaultCompleted int
				Jobs             []struct {
					ID      string
					Queue   string
					Command string
					Status  string
					Created string
				}
			}{
				DefaultPending:   10,
				DefaultRunning:   2,
				DefaultCompleted: 100,
				Jobs: []struct {
					ID      string
					Queue   string
					Command string
					Status  string
					Created string
				}{
					{
						ID:      "123",
						Queue:   "default",
						Command: "echo hello",
						Status:  "completed",
						Created: time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
					},
					{
						ID:      "124",
						Queue:   "default",
						Command: "python script.py",
						Status:  "running",
						Created: time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
					},
				},
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
