# QQ - Simple and Fast Job Queue

QQ is a simple, fast job queue based on [River Queue](https://riverqueue.com/docs), [Cobra](https://github.com/spf13/cobra), and [Viper](https://github.com/spf13/viper). It is designed to be used in a distributed environment where you have multiple workers running on different machines. All data persistence is done using PostgreSQL.

## Features

- Queue, monitor, and execute bash commands reliably
- Distributed architecture with shared PostgreSQL database
- Command-line interface for all operations
- Priority scheduling
- Job execution with output capture
- Future job scheduling
- Web UI for monitoring queue status

## Installation

```bash
go install github.com/yourname/qq@latest
```

Or build from source:

```bash
git clone https://github.com/yourname/qq.git
cd qq
go build
```

## Quick Start

### 1. Set up PostgreSQL

Make sure you have a PostgreSQL instance running. QQ will use this database to store jobs and queue information.

### 2. Initialize the Database Schema

```bash
qq init --db-url=postgres://user:password@localhost:5432/yourdb
```

### 3. Start a Worker

```bash
qq worker --db-url=postgres://user:password@localhost:5432/yourdb
```

### 4. Add a Job

```bash
qq job add "echo hello world" --priority=1
```

### 5. Monitor Jobs

```bash
qq job ls
```

### 6. Start the Web UI

```bash
qq server --addr=:8080
```

Then open [http://localhost:8080](http://localhost:8080) in your browser.

## Commands

QQ is implemented as a single binary with the following CLI commands:

- `qq worker` - Starts a worker that listens for jobs on the queue and processes them.
- `qq server` - Starts a server that shows queue status.
- `qq job add|rm|ls` - Subcommands for managing jobs.
- `qq queue add|rm|ls` - Subcommands for managing queues.
- `qq init` - Initialize the database schema.

All workers and servers connect to the same PostgreSQL database to coordinate.

## Configuration

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

## License

MIT