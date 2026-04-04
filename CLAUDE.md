# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is qq

qq is a distributed job queue CLI built on River Queue, Cobra, and Viper. It runs shell commands as jobs, coordinated through PostgreSQL. A single binary provides worker, server (web dashboard), and job/queue management commands. There is also a Python client SDK that connects directly to the same database.

## Build & Test Commands

```bash
# Build
task build              # Builds to dist/qq
go build                # Builds to ./qq

# Tests (uses Taskfile.dev)
task test:unit          # Unit tests only (-short flag, no Docker needed)
task test:integration   # Integration tests (requires Docker for temp Postgres)
task test:all           # All tests including Python client
task test:python        # Python client tests only

# Run a single test
go test -run TestName ./pkg/queue/
go test -short ./...    # Skip integration tests

# Database setup
task init               # Install river CLI + run migrations
```

## Architecture

### Command → Queue → Database flow

CLI commands (`cmd/`) parse flags and config, then call into `pkg/queue/queue.go` which is the central business logic layer. The queue package wraps River Queue's client, manages the custom `job_results` table, and handles job execution via `BashWorker`.

**Job execution path:** `AddJob()` → River inserts into `river_job` → `BashWorker.Work()` runs `bash -c <command>` → captures stdout/stderr + exit code → saves to `job_results` table.

### Config resolution order

1. CLI flags (`--db-url`)
2. Environment variables (`DATABASE_URL`)
3. Config file (`.qq.yaml` in current dir or home dir)

Viper handles merging. Commands read values via `viper.GetString()` etc.

### Database schema

Two table sets: River's internal tables (created via `rivermigrate`) and a custom `job_results` table (stores output, exit code per job attempt). The `qq init` command runs both River migrations and the custom table creation.

**Job state mapping:** River states (`available`, `scheduled`) → `pending`; `running` → `running`; `completed` → `completed`; (`discarded`, `cancelled`, `retryable`) → `failed`.

### Version injection

GoReleaser injects version at build time via `-X qq/cmd.Version={{.Version}}`.

### Test patterns

- Integration tests use `testutils.SetupTestDatabase()` which spins up a temporary Docker Postgres container
- Integration tests are skipped with `testing.Short()` — use `-short` flag to skip them
- Uses `testify` (assert/require) for assertions
- End-to-end tests in `pkg/integration_test.go` spawn the actual CLI binary via `exec.Command()`

### Python client (`clients/python/`)

Direct PostgreSQL access via psycopg v3. Creates per-operation connections (no pooling). Inserts directly into `river_job` table with the same JSON args format as the Go side.
