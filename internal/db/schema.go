package db

import (
	"context"
	"fmt"
)

// schema replaces SQLAlchemy's Base.metadata.create_all. There is no ORM here
// to generate DDL, so the statements are explicit.
//
// TODO for myself: this is fine for a fresh dev database and nothing else. It
// will not alter an existing table, so the moment the schema changes (and the
// TODOs in internal/models all change it) a database with data in it silently
// drifts. Replace with real migration files, golang-migrate or goose, and run
// them instead of this.
//
// TODO for myself (6): every execution_id / current_worker_id column below
// needs an index. Postgres does not index foreign key columns automatically,
// so the attempt and event timeline reads are sequential scans as written.
const schema = `
CREATE TABLE IF NOT EXISTS workers (
    worker_id       bigserial   PRIMARY KEY,
    current_state   text        NOT NULL,
    task_queue_name text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS executions (
    execution_id      bigserial   PRIMARY KEY,
    current_worker_id bigint      REFERENCES workers (worker_id),
    state             text        NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS attempts (
    attempt_id   bigserial   PRIMARY KEY,
    execution_id bigint      NOT NULL REFERENCES executions (execution_id),
    worker_id    bigint      NOT NULL REFERENCES workers (worker_id),
    attempt_num  integer     NOT NULL,
    state        text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    error        text,
    finished_at  timestamptz,
    CONSTRAINT uq_execution_attempt_num UNIQUE (execution_id, attempt_num)
);

CREATE TABLE IF NOT EXISTS events (
    event_id     bigserial   PRIMARY KEY,
    execution_id bigint      NOT NULL REFERENCES executions (execution_id),
    event_type   text        NOT NULL,
    payload      json,
    created_at   timestamptz NOT NULL DEFAULT now()
);
`

// InitSchema creates every table if it does not already exist.
//
// TODO for myself: SQLAlchemy kept executions.updated_at fresh with
// onupdate=func.now(), which was ORM behaviour, not a database default. There
// is no ORM here, so every UPDATE must set updated_at = now() itself, or the
// column needs a trigger. Easy to forget and silently wrong.
func InitSchema(ctx context.Context) error {
	if pool == nil {
		return fmt.Errorf("db.Init must be called before InitSchema")
	}

	if _, err := pool.Exec(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	return nil
}
