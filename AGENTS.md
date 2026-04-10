# OpenCode Agent Instructions

## Quick Start & Environment
- **Build**: `go build` (produces `./qq`) or `task build` (produces `dist/qq`)
- **Database Setup**: `task init` (Installs river CLI + runs migrations)
- **Testing**:
  - Unit tests: `task test:unit` or `go test -short ./...`
  - Integration tests: `task test:integration` (Requires Docker for Postgres)
  - Python client tests: `task test:python`

## Core Architecture & Flow
- **Job Execution Path**: `AddJob()` $\to$ River insertion $\to$ `BashWorker.Work()` (runs `bash -c <command>`) $\to$ result saved in `job_results` table.
- **Config Resolution Order**: 
  1. CLI flags (`--db-url`)
  2. Environment variables (`DATABASE_URL`)
  3. Config file (`.qq.yaml` in current or home dir)

## Key Components
- **Go Core**: `cmd/` (CLI entrypoints), `pkg/queue/` (Business logic), `pkg/worker/` (Bash execution).
- **Python Client**: Located in `clients/python/`. Uses `psycopg v3` for direct DB access.

## Development Standards
- **Linting/Typechecking**: Run project lint commands before committing.
- **Testing Pattern**: Use `testutils.SetupTestDatabase()` for integration tests.
- **Job States Mapping**: 
  - `available`/`scheduled` $\to$ `pending`
  - `running` $\to$ `running`
  - `completed` $\to$ `completed`
  - `discarded`/`cancelled`/`retryable` $\to$ `failed`
