# qq

qq is a simple, fast job queue based on River Queue (https://riverqueue.com/docs), Cobra and Viper. It is designed to be used in a distributed environment where you have multiple workers running on different machines. All data persistence is done using PostgreSQL.

## Components
qq is implemented as a single binary with the following CLI commands:

- `qq init` - Initialize the database schema (must be run before using other commands).
- `qq worker` - Starts a worker that listens for jobs on the queue and processes them.
- `qq server` - Starts a server that shows queue status.
- `qq job add|rm|ls` - Subcommands for managing jobs.
- `qq queue add|rm|ls` - Subcommands for managing queues.

All workers and servers connect to the same PostgreSQL database to coordinate. The Python client SDK can also connect to this same database.

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

- `clients/` - Client SDKs for different languages
  - `python/` - Python client SDK
    - Uses psycopg (PostgreSQL driver)
    - Direct database access

### Key Commands

```bash
# Build the project
go build

# Run the application
./qq [command]
```

### Usage Workflow

1. Initialize the database schema: 
   ```bash
   # Using command-line flags
   qq init --db-url=postgres://user:password@localhost:5432/yourdb
   
   # Or using environment variables
   export DATABASE_URL=postgres://user:password@localhost:5432/yourdb
   qq init
   ```

2. Start a worker:
   ```bash
   # Using command-line flags
   qq worker --db-url=postgres://user:password@localhost:5432/yourdb --concurrency=5
   
   # Or using environment variables
   export DATABASE_URL=postgres://user:password@localhost:5432/yourdb
   qq worker --concurrency=5
   ```

3. Add jobs to the queue:
   ```bash
   qq job add "echo hello world" --priority=1
   ```

4. Start the monitoring server:
   ```bash
   qq server --addr=:8080
   ```

5. Use the Python client (if needed):
   ```python
   from qq import QQClient
   
   # Initialize with explicit URL
   client = QQClient(db_url="postgres://user:password@localhost:5432/yourdb")
   
   # Or using environment variable
   # export DATABASE_URL=postgres://user:password@localhost:5432/yourdb
   # client = QQClient()
   
   # Add a job
   job = client.add_job(command="echo 'Hello from Python!'")
   print(f"Added job: {job['id']}")
   ```

### River Queue Implementation Notes

- Uses a "bash_command" job type to run shell commands
- Jobs can be scheduled with priorities (lower numbers = higher priority)
- Jobs can be scheduled for future execution using the `--schedule` flag
- River Queue schema is created in the database without schema qualification
  - Tables include: `river_job` and `job_results`
- Job execution is handled by the worker process
- Output from job execution is captured and stored in the `job_results` table

### Database Schema

When initializing the database with `qq init`, the following tables are created:

- `river_job` - River's internal job table that stores all job information
- `job_results` - Custom table for storing command outputs, exit codes, and errors

### Configuration

QQ can be configured via:
1. Command-line flags
2. Environment variables 
3. Configuration file (.qq.yaml)

By default, QQ looks for a `.qq.yaml` file in the current directory or home directory.

#### Database Configuration

You can configure the database connection using any of these methods:

- Command-line flag: `--db-url=postgres://user:password@localhost:5432/yourdb`
- Environment variable: `DATABASE_URL=postgres://user:password@localhost:5432/yourdb`
- Configuration file: See example below

#### Example Configuration File

```yaml
db_url: postgres://user:password@localhost:5432/yourdb

worker:
  concurrency: 5
  queue: default
  interval: 5

server:
  address: :8080
```

#### Environment Variables

When using environment variables:

```bash
# Set database URL
export DATABASE_URL=postgres://user:password@localhost:5432/yourdb

# Then run commands without specifying db-url
qq init
qq worker
```