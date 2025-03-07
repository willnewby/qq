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

### Pre-built binaries

Download the pre-built binaries from the [releases page](https://github.com/willnewby/qq/releases).

#### macOS

```bash
# For Intel Macs (x86_64)
curl -L https://github.com/willnewby/qq/releases/latest/download/qq_Darwin_x86_64.tar.gz | tar xz
sudo mv qq /usr/local/bin/

# For M1/M2 Macs (arm64)
curl -L https://github.com/willnewby/qq/releases/latest/download/qq_Darwin_arm64.tar.gz | tar xz
sudo mv qq /usr/local/bin/
```

#### Linux

```bash
# For x86_64
curl -L https://github.com/willnewby/qq/releases/latest/download/qq_Linux_x86_64.tar.gz | tar xz
sudo mv qq /usr/local/bin/

# For arm64
curl -L https://github.com/willnewby/qq/releases/latest/download/qq_Linux_arm64.tar.gz | tar xz
sudo mv qq /usr/local/bin/
```

#### Using Docker

```bash
docker pull ghcr.io/willnewby/qq:latest
docker run --rm ghcr.io/willnewby/qq:latest --help
```

### Install via package managers

#### Debian/Ubuntu

```bash
# Download the .deb package
curl -LO https://github.com/willnewby/qq/releases/latest/download/qq_amd64.deb
# Install the package
sudo dpkg -i qq_amd64.deb
```

#### RHEL/CentOS

```bash
# Download the .rpm package
curl -LO https://github.com/willnewby/qq/releases/latest/download/qq_amd64.rpm
# Install the package
sudo rpm -i qq_amd64.rpm
```

### Build from source

```bash
# Requires Go 1.23 or later
git clone https://github.com/willnewby/qq.git
cd qq
go build
```

## Quick Start

### 1. Set up PostgreSQL

Make sure you have a PostgreSQL instance running. QQ will use this database to store jobs and queue information.

### 2. Initialize the Database Schema

Using command-line flags:
```bash
qq init --db-url=postgres://user:password@localhost:5432/yourdb
```

Or using environment variables:
```bash
export DATABASE_URL=postgres://user:password@localhost:5432/yourdb
qq init
```

### 3. Start a Worker

Using command-line flags:
```bash
qq worker --db-url=postgres://user:password@localhost:5432/yourdb
```

Or using environment variables:
```bash
export DATABASE_URL=postgres://user:password@localhost:5432/yourdb
qq worker
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

QQ can be configured via:
1. Command-line flags
2. Environment variables 
3. Configuration file

By default, QQ looks for a `.qq.yaml` file in the current directory or home directory.

### Database Configuration

You can configure the database connection using any of these methods:

- Command-line flag: `--db-url=postgres://user:password@localhost:5432/yourdb`
- Environment variable: `DATABASE_URL=postgres://user:password@localhost:5432/yourdb`
- Configuration file: See example below

### Example Configuration File

```yaml
db_url: postgres://user:password@localhost:5432/yourdb

worker:
  concurrency: 5
  queue: default
  interval: 5

server:
  address: :8080
```

### Environment Variables

When using environment variables:

```bash
# Set database URL
export DATABASE_URL=postgres://user:password@localhost:5432/yourdb

# Then run commands without specifying db-url
qq init
qq worker
```

## License

MIT