package migration

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The River schema migration SQL
// This is a simplified version of the schema River creates
const riverSchemaSql = `
-- Create River queue related tables
CREATE SCHEMA IF NOT EXISTS river;

-- Create jobs table
CREATE TABLE IF NOT EXISTS river.jobs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    kind TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 1,
    queue TEXT NOT NULL DEFAULT 'default',
    state TEXT NOT NULL DEFAULT 'available',
    scheduled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    args JSONB NOT NULL DEFAULT '{}'::JSONB,
    attempted_at TIMESTAMPTZ,
    attempted_by TEXT[],
    attempt INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 10,
    errors JSONB DEFAULT NULL,
    finalized_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::JSONB,
    tags TEXT[] DEFAULT '{}'::TEXT[],
    unique_key BYTEA,
    unique_states TEXT[] DEFAULT '{}'::TEXT[]
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_river_jobs_kind ON river.jobs (kind);
CREATE INDEX IF NOT EXISTS idx_river_jobs_queue ON river.jobs (queue);
CREATE INDEX IF NOT EXISTS idx_river_jobs_state ON river.jobs (state);
CREATE INDEX IF NOT EXISTS idx_river_jobs_priority ON river.jobs (priority);
CREATE INDEX IF NOT EXISTS idx_river_jobs_scheduled_at ON river.jobs (scheduled_at);
CREATE INDEX IF NOT EXISTS idx_river_jobs_unique_key_kind ON river.jobs (unique_key, kind) WHERE unique_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_river_jobs_state_queue_priority_scheduled_at ON river.jobs (state, queue, priority, scheduled_at)
    WHERE state = 'available';

-- Create leader table for job coordination
CREATE TABLE IF NOT EXISTS river.leaders (
    shard_index INTEGER NOT NULL,
    client_id TEXT NOT NULL,
    leadership_ends_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (shard_index)
);

-- Create queues table for tracking queues
CREATE TABLE IF NOT EXISTS river.queues (
    name TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create notifications table for job events
CREATE TABLE IF NOT EXISTS river.notifications (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    payload JSONB NOT NULL DEFAULT '{}'::JSONB
);
`

// CreateSchema creates the River Queue schema in the database
func CreateSchema(ctx context.Context, pool *pgxpool.Pool) error {
	// Execute the schema SQL
	_, err := pool.Exec(ctx, riverSchemaSql)
	if err != nil {
		return fmt.Errorf("failed to create River schema: %w", err)
	}

	// Verify by trying to insert a default queue
	_, err = pool.Exec(ctx, `
		INSERT INTO river.queues (name)
		VALUES ('default')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("failed to initialize default queue: %w", err)
	}

	return nil
}