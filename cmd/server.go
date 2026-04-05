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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"qq/pkg/config"
	"qq/pkg/database"
	"qq/pkg/queue"
)

const commonCSS = `
	body { font-family: Arial, sans-serif; margin: 0; padding: 20px; }
	h1 { color: #333; }
	h2 { color: #444; }
	a { color: #0366d6; text-decoration: none; }
	a:hover { text-decoration: underline; }
	table { border-collapse: collapse; width: 100%; margin-bottom: 20px; }
	th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
	th { background-color: #f2f2f2; }
	tr:nth-child(even) { background-color: #f9f9f9; }
	.nav { margin-bottom: 20px; }
	.nav a { margin-right: 12px; }
	.output { background: #1e1e1e; color: #d4d4d4; padding: 16px; border-radius: 4px; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word; font-family: monospace; font-size: 13px; }
	.status-completed { color: #22863a; font-weight: bold; }
	.status-running { color: #0366d6; font-weight: bold; }
	.status-pending { color: #b08800; font-weight: bold; }
	.status-failed { color: #cb2431; font-weight: bold; }
	.meta { margin-bottom: 20px; }
	.meta dt { font-weight: bold; display: inline; }
	.meta dd { display: inline; margin-left: 4px; margin-right: 16px; }
`

const dashboardTmpl = `<!DOCTYPE html>
<html>
<head>
	<title>QQ - Queue Dashboard</title>
	<style>` + commonCSS + `</style>
</head>
<body>
	<h1>QQ - Queue Dashboard</h1>
	<p class="nav"><a href="/">Refresh</a></p>

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
			<td><a href="/queue/{{.Name}}">{{.Name}}</a></td>
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
			<td><a href="/job/{{.ID}}">{{.ID}}</a></td>
			<td><a href="/queue/{{.Queue}}">{{.Queue}}</a></td>
			<td>{{.Command}}</td>
			<td><span class="status-{{.Status}}">{{.Status}}</span></td>
			<td>{{.Created}}</td>
		</tr>
		{{end}}
	</table>
</body>
</html>`

const queueTmpl = `<!DOCTYPE html>
<html>
<head>
	<title>QQ - Queue: {{.QueueName}}</title>
	<style>` + commonCSS + `</style>
</head>
<body>
	<h1>Queue: {{.QueueName}}</h1>
	<p class="nav"><a href="/">← Dashboard</a> <a href="/queue/{{.QueueName}}">Refresh</a></p>

	{{if .Stats}}
	<h2>Stats</h2>
	<table>
		<tr>
			<th>Pending</th>
			<th>Running</th>
			<th>Completed</th>
			<th>Failed</th>
		</tr>
		<tr>
			<td>{{.Stats.Pending}}</td>
			<td>{{.Stats.Running}}</td>
			<td>{{.Stats.Completed}}</td>
			<td>{{.Stats.Failed}}</td>
		</tr>
	</table>
	{{end}}

	<h2>Jobs</h2>
	{{if .Jobs}}
	<table>
		<tr>
			<th>ID</th>
			<th>Command</th>
			<th>Status</th>
			<th>Exit Code</th>
			<th>Created</th>
		</tr>
		{{range .Jobs}}
		<tr>
			<td><a href="/job/{{.ID}}">{{.ID}}</a></td>
			<td>{{.Command}}</td>
			<td><span class="status-{{.Status}}">{{.Status}}</span></td>
			<td>{{.ExitCode}}</td>
			<td>{{.Created}}</td>
		</tr>
		{{end}}
	</table>
	{{else}}
	<p>No jobs found in this queue.</p>
	{{end}}
</body>
</html>`

const jobTmpl = `<!DOCTYPE html>
<html>
<head>
	<title>QQ - Job {{.ID}}</title>
	<style>` + commonCSS + `</style>
</head>
<body>
	<h1>Job {{.ID}}</h1>
	<p class="nav"><a href="/">← Dashboard</a> <a href="/queue/{{.Queue}}">← Queue: {{.Queue}}</a> <a href="/job/{{.ID}}">Refresh</a></p>

	<dl class="meta">
		<dt>Queue:</dt><dd><a href="/queue/{{.Queue}}">{{.Queue}}</a></dd>
		<dt>Command:</dt><dd><code>{{.Command}}</code></dd>
		<dt>Status:</dt><dd><span class="status-{{.Status}}">{{.Status}}</span></dd>
		<dt>Exit Code:</dt><dd>{{.ExitCode}}</dd>
		<dt>Attempt:</dt><dd>{{.Attempt}}</dd>
		<dt>Created:</dt><dd>{{.Created}}</dd>
		<dt>Scheduled:</dt><dd>{{.Scheduled}}</dd>
	</dl>

	<h2>Output</h2>
	{{if .Output}}
	<pre class="output">{{.Output}}</pre>
	{{else}}
	<p>No output available.</p>
	{{end}}
</body>
</html>`

func mapJobStatus(state string) string {
	switch state {
	case "available", "scheduled", "retryable":
		return "pending"
	case "running":
		return "running"
	case "completed":
		return "completed"
	case "discarded", "cancelled":
		return "failed"
	default:
		return "unknown"
	}
}

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

		mux := http.NewServeMux()

		// Dashboard handler
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}

			queueClient, err := queue.NewQueueClient(ctx, db)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create queue client: %v", err), http.StatusInternalServerError)
				return
			}
			defer queueClient.Close(ctx)

			queueStats, err := queueClient.GetQueueStats(ctx, "")
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get queue stats: %v", err), http.StatusInternalServerError)
				return
			}

			if len(queueStats) == 0 {
				queueStats = append(queueStats, queue.QueueStats{
					Name:      "default",
					Pending:   0,
					Running:   0,
					Completed: 0,
					Failed:    0,
				})
			}

			jobs, err := queueClient.ListJobs(ctx, "", "", 100)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to list jobs: %v", err), http.StatusInternalServerError)
				return
			}

			type templateJob struct {
				ID      string
				Queue   string
				Command string
				Status  string
				Created string
			}

			var templateJobs []templateJob
			for _, job := range jobs {
				templateJobs = append(templateJobs, templateJob{
					ID:      fmt.Sprintf("%d", job.ID),
					Queue:   job.Queue,
					Command: job.Command,
					Status:  mapJobStatus(job.State),
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

			t, err := template.New("dashboard").Parse(dashboardTmpl)
			if err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
				return
			}

			if err := t.Execute(w, data); err != nil {
				http.Error(w, "Template execution error", http.StatusInternalServerError)
				return
			}
		})

		// Queue detail handler: /queue/{name}
		mux.HandleFunc("/queue/", func(w http.ResponseWriter, r *http.Request) {
			queueName := strings.TrimPrefix(r.URL.Path, "/queue/")
			if queueName == "" {
				http.Redirect(w, r, "/", http.StatusFound)
				return
			}

			queueClient, err := queue.NewQueueClient(ctx, db)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create queue client: %v", err), http.StatusInternalServerError)
				return
			}
			defer queueClient.Close(ctx)

			stats, err := queueClient.GetQueueStats(ctx, queueName)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to get queue stats: %v", err), http.StatusInternalServerError)
				return
			}

			var queueStat *queue.QueueStats
			if len(stats) > 0 {
				queueStat = &stats[0]
			}

			jobs, err := queueClient.ListJobs(ctx, queueName, "", 100)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to list jobs: %v", err), http.StatusInternalServerError)
				return
			}

			type templateJob struct {
				ID       string
				Command  string
				Status   string
				ExitCode int
				Created  string
			}

			var templateJobs []templateJob
			for _, job := range jobs {
				templateJobs = append(templateJobs, templateJob{
					ID:       fmt.Sprintf("%d", job.ID),
					Command:  job.Command,
					Status:   mapJobStatus(job.State),
					ExitCode: job.ExitCode,
					Created:  job.CreatedAt.Format(time.RFC3339),
				})
			}

			data := struct {
				QueueName string
				Stats     *queue.QueueStats
				Jobs      []templateJob
			}{
				QueueName: queueName,
				Stats:     queueStat,
				Jobs:      templateJobs,
			}

			t, err := template.New("queue").Parse(queueTmpl)
			if err != nil {
				http.Error(w, "Template error", http.StatusInternalServerError)
				return
			}

			if err := t.Execute(w, data); err != nil {
				http.Error(w, "Template execution error", http.StatusInternalServerError)
				return
			}
		})

		// Job detail handler: /job/{id}
		mux.HandleFunc("/job/", func(w http.ResponseWriter, r *http.Request) {
			idStr := strings.TrimPrefix(r.URL.Path, "/job/")
			jobID, err := strconv.ParseInt(idStr, 10, 64)
			if err != nil {
				http.Error(w, "Invalid job ID", http.StatusBadRequest)
				return
			}

			queueClient, err := queue.NewQueueClient(ctx, db)
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to create queue client: %v", err), http.StatusInternalServerError)
				return
			}
			defer queueClient.Close(ctx)

			job, err := queueClient.GetJob(ctx, jobID)
			if err != nil {
				http.Error(w, "Job not found", http.StatusNotFound)
				return
			}

			data := struct {
				ID        string
				Queue     string
				Command   string
				Status    string
				ExitCode  int
				Attempt   int
				Created   string
				Scheduled string
				Output    string
			}{
				ID:        fmt.Sprintf("%d", job.ID),
				Queue:     job.Queue,
				Command:   job.Command,
				Status:    mapJobStatus(job.State),
				ExitCode:  job.ExitCode,
				Attempt:   job.Attempt,
				Created:   job.CreatedAt.Format(time.RFC3339),
				Scheduled: job.ScheduledAt.Format(time.RFC3339),
				Output:    job.Output,
			}

			t, err := template.New("job").Parse(jobTmpl)
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
			Addr:    addr,
			Handler: mux,
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
