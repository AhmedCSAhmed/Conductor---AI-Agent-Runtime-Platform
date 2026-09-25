// Package models holds the database row types. These are plain structs
// scanned from pgx, not an ORM; the schema itself lives in internal/db.
//
// TODO for myself (5): states are bare strings everywhere in this file
// (Worker.CurrentState, Execution.State, Attempt.State). Define
// `type ExecutionState string` and `type AttemptState string` with the valid
// values as constants, and back them with a CHECK constraint in the schema.
// Do this before three packages invent three different spellings of "failed".
package models

import (
	"encoding/json"
	"time"
)

// Worker is one process polling a Temporal task queue.
type Worker struct {
	// TODO for myself (8): a database-assigned serial id is wrong for
	// workers. A worker is an external process that restarts; if the DB
	// assigns the id it can never re-register as itself. Switch to a string
	// or UUID supplied by the worker process at startup.
	// TODO for myself (9): add LastHeartbeatAt so a dead worker is
	// detectable and its in-flight attempts can be reaped. CreatedAt alone
	// cannot tell me whether this worker is still alive.
	WorkerID int64 `json:"worker_id"`

	CurrentState  string `json:"current_state"`
	TaskQueueName string `json:"task_queue_name"`

	CreatedAt time.Time `json:"created_at"`
}

// Execution is one agent run.
//
// TODO for myself (1): HIGHEST PRIORITY. Nothing links this row back to the
// Temporal workflow that owns it. Add:
//
//	WorkflowID string  // unique, indexed
//	RunID      *string
//
// and populate both when the workflow starts. Without them I cannot cancel a
// run, query a live workflow, or reconcile this table against Temporal after
// a crash, which is the entire point of the project.
type Execution struct {
	ExecutionID int64 `json:"execution_id"`

	// CurrentWorkerID is nil until a worker picks the execution up.
	CurrentWorkerID *int64 `json:"current_worker_id"`

	State string `json:"state"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Attempt is one try at an Execution.
type Attempt struct {
	AttemptID   int64 `json:"attempt_id"`
	ExecutionID int64 `json:"execution_id"`
	WorkerID    int64 `json:"worker_id"`

	// TODO for myself (10): uq_execution_attempt_num in the schema enforces
	// uniqueness but nothing allocates this value. Needs a
	// SELECT max(attempt_num) + 1 in the same transaction as the insert,
	// otherwise two concurrent attempts race into a duplicate key error.
	AttemptNum int `json:"attempt_num"`

	State string `json:"state"`

	CreatedAt time.Time `json:"created_at"`

	// Error is set when the attempt failed.
	Error *string `json:"error"`

	// FinishedAt is nil while the attempt is still running.
	FinishedAt *time.Time `json:"finished_at"`
}

// Event is an append-only audit record for an Execution.
type Event struct {
	EventID     int64  `json:"event_id"`
	ExecutionID int64  `json:"execution_id"`
	EventType   string `json:"event_type"`

	// TODO for myself (7): the column is `json`; make it `jsonb`. This is an
	// append-only trail I will want to query, and only jsonb supports GIN
	// indexing.
	Payload json.RawMessage `json:"payload"`

	CreatedAt time.Time `json:"created_at"`
}
