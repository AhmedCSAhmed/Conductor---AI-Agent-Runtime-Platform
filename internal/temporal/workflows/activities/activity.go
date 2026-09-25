// Package activities holds the activity implementations. Activities are the
// only place in the Temporal layer allowed to do I/O.
package activities

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/activity"

	"github.com/AhmedCSAhmed/conductor/internal/db"
)

// AddExecution creates the Execution row for a run.
//
// TODO for myself (2): activities must assume they run more than once.
// Temporal retries on any error, on worker crash, and on timeout. Make this
// idempotent (upsert on workflow_id) before it is load bearing.
//
// TODO for myself (4): `name` is decorative. It gets logged and thrown away;
// CreateExecution is called with the default state and nothing else. Either
// use it or drop the parameter.
//
// TODO for myself: add activity.RecordHeartbeat(ctx) once this does real work,
// plus a HeartbeatTimeout in the caller's ActivityOptions, so a stalled call is
// detected instead of hanging until StartToCloseTimeout expires.
func AddExecution(ctx context.Context, name string) (int64, error) {
	executionID, err := db.CreateExecution(ctx, db.StatePending)
	if err != nil {
		return 0, fmt.Errorf("add execution for %q: %w", name, err)
	}

	activity.GetLogger(ctx).Info("created execution",
		"execution_id", executionID,
		"name", name,
	)

	return executionID, nil
}

// TODO for myself: still need UpdateExecution, GetExecutionByID,
// ListExecutions, CreateAttempt, FinishAttempt, AppendEvent as activities,
// each one a thin wrapper over internal/db. Keep them thin: retry policy and
// sequencing belong in the workflow, not in here.
