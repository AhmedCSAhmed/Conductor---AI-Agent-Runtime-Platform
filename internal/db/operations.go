package db

import (
	"context"
	"fmt"

	"github.com/AhmedCSAhmed/conductor/internal/models"
)

// StatePending is the state a freshly created Execution starts in.
//
// TODO for myself (5): this is the only state constant that exists. It belongs
// in a real ExecutionState type in internal/models alongside the others.
const StatePending = "PENDING"

// CreateExecution inserts a new Execution row and returns its id.
//
// TODO for myself (2): this insert is not idempotent and the workflow retries
// it up to 3 times. If the INSERT commits but the worker dies before the
// result is reported back to Temporal, the activity is retried and I get a
// second Execution row for the same run. Take a workflowID and make this an
// INSERT ... ON CONFLICT (workflow_id) DO UPDATE ... RETURNING execution_id,
// which needs the column from TODO (1) first.
func CreateExecution(ctx context.Context, state string) (int64, error) {
	if pool == nil {
		return 0, fmt.Errorf("db.Init must be called before CreateExecution")
	}

	if state == "" {
		state = StatePending
	}

	const query = `
		INSERT INTO executions (state)
		VALUES ($1)
		RETURNING execution_id
	`

	var executionID int64
	if err := pool.QueryRow(ctx, query, state).Scan(&executionID); err != nil {
		return 0, fmt.Errorf("insert execution: %w", err)
	}

	return executionID, nil
}

// TODO for myself: still need the rest of the repository layer, in roughly the
// order the API and the workflow will want them:
//
//	GetExecution(ctx, executionID) (*models.Execution, error)

var err error
func GetExecution(ctx context.Context, executionID int64) (*models.Execution, error) {
	poll, err := pool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	defer poll.Release()

	const query = `
		SELECT execution_id, state, created_at, updated_at
		FROM executions
		WHERE execution_id = $1
	` 

	rows, err  := poll.Query(ctx, query, executionID)
	if err != nil {
		return nil, fmt.Errorf("query execution: %w", err)
	}

	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("rows error: %w", err)
		}
		return nil, fmt.Errorf("execution not found")
	}

	var execution models.Execution
	if err := rows.Scan(&execution.ExecutionID, &execution.State, &execution.CreatedAt, &execution.UpdatedAt); err != nil {
		return nil, fmt.Errorf("scan execution: %w", err)
	}
	
	return &execution, nil
}

//	ListExecutions(ctx, limit, offset) ([]models.Execution, error)
//	UpdateExecutionState(ctx, executionID, state) error  // must set updated_at
//	CreateAttempt(ctx, executionID, workerID) (*models.Attempt, error)  // allocates attempt_num
//	FinishAttempt(ctx, attemptID, state string, attemptErr error) error
//	AppendEvent(ctx, executionID, eventType string, payload any) error
//	RegisterWorker(ctx, ...) / HeartbeatWorker(ctx, ...)
//
// Go has no ORM here, so each of these is explicit SQL. Return
// pgx.ErrNoRows-wrapped sentinels the API layer can map to a 404 rather than
// leaking pgx errors upward.
