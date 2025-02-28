# qq

qq is a simple, fast job queue based on River Queue (https://riverqueue.com/docs), Cobra and Viper. It is designed to be used in a distributed environment where you have multiple workers running on different machines. All data persistence is done using Postgres.

## Components
qq is implemented as a single binary with the following CLI commands:

- `qq init` - Initialize the database schema (must be run before using other commands).
- `qq worker` - Starts a worker that listens for jobs on the queue and processes them.
- `qq server` - Starts a server that shows queue status.
- `qq job add|rm|ls` - Subcommands for managing jobs.
- `qq queue add|rm|ls` - Subcommands for managing queues.

All workers and servers connect to the same Postgres database to coordinate.

## Development Information

### Project Structure

- `cmd/` - Command definitions using Cobra
  - `root.go` - Root command and global flags
  - `worker.go` - Worker command implementation
  - `server.go` - Server command implementation
  - `job.go`, `add.go`, `ls.go`, `rm.go` - Job management commands
  - `queue.go` - Queue management commands
  - `init.go` - Database initialization command

- `pkg/` - Core functionality packages
  - `config/` - Configuration handling
  - `database/` - Database connection management
  - `migration/` - Database schema management
  - `queue/` - River Queue integration

### Key Commands

```bash
# Build the project
go build

# Run the application
./qq [command]
```

### Usage Workflow

1. Initialize the database schema: 
   ```
   qq init --db-url=postgres://user:password@localhost:5432/yourdb
   ```

2. Start a worker:
   ```
   qq worker --db-url=postgres://user:password@localhost:5432/yourdb --concurrency=5
   ```

3. Add jobs to the queue:
   ```
   qq job add "echo hello world" --priority=1
   ```

4. Start the monitoring server:
   ```
   qq server --addr=:8080
   ```

### River Queue Implementation Notes

- Uses a "bash_command" job type to run shell commands
- Jobs can be scheduled with priorities (lower numbers = higher priority)
- Jobs can be scheduled for future execution using the `--schedule` flag
- River Queue schema is created in the "river" schema in PostgreSQL
- Job execution is handled by the worker process
- Output from job execution is captured and displayed

### Configuration

QQ can be configured via command-line flags or a configuration file. By default, QQ looks for a `.qq.yaml` file in the current directory or home directory.

Example configuration file:

```yaml
db_url: postgres://user:password@localhost:5432/yourdb

worker:
  concurrency: 5
  queue: default
  interval: 5

server:
  address: :8080
```