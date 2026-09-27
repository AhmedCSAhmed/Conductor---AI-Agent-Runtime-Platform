package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/AhmedCSAhmed/conductor/internal/models"
)

// StatePending is the state a freshly created Execution starts in.
const StatePending = "PENDING"

// AttemptStateCreated is the state a freshly created Attempt starts in.
const AttemptStateCreated = "CREATED"

// CreateExecution inserts a new Execution row and returns its id.
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

// GetExecution returns the Execution with the given id.
func GetExecution(ctx context.Context, executionID int64) (*models.Execution, error) {
	poll, err := pool.Acquire(ctx)
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

// ListExecutions returns up to limit Executions, ordered newest first.
func ListExecutions(ctx context.Context, limit, offset int) ([]models.Execution, error) {
	poll, err := pool.Acquire(ctx) // Acquire a connection from the pool
	if err != nil {
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	defer poll.Release()

	const query = `
		SELECT execution_id, state, created_at, updated_at
		FROM executions
		ORDER BY execution_id
		LIMIT $1 OFFSET $2
	`

	rows, err := poll.Query(ctx, query, limit, offset)
	if err != nil{
		return nil, fmt.Errorf("query executions: %w", err)
	}

	defer rows.Close()

	var executions []models.Execution
	for rows.Next(){
		var execution models.Execution
		if err := rows.Scan(&execution.ExecutionID, &execution.State, &execution.CreatedAt, &execution.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}

		executions = append(executions, execution)
	}

	// Check for any errors that occurred during iteration
	if err := rows.Err(); err != nil { 
			return nil, fmt.Errorf("rows error: %w", err)
		}
	return executions, nil // Return the list of executions and nil error
}



// UpdateExecutionState sets an Execution's state and refreshes updated_at.
func UpdateExecutionState(ctx context.Context, executionID int64, state string) error {
	poll, err := pool.Acquire(ctx)
	if err != nil{ 
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer poll.Release()

	const query = `
		UPDATE executions
		SET state = $1, updated_at = NOW()
		WHERE execution_id = $2
	`

	if _, err := poll.Exec(ctx, query, state, executionID); err != nil {
		return fmt.Errorf("update execution state: %w", err)
	}
	return nil
} 

// CreateAttempt inserts a new Attempt for an Execution, allocating its attempt_num.
func CreateAttempt(ctx context.Context, executionID, workerID int64) (*models.Attempt, error){
	poll, err :=  pool.Acquire(ctx)
	if err != nil{
		return nil, fmt.Errorf("acquire connection: %w", err)
	}

	defer poll.Release() // always release the connection back to the pool, right now we are deffering it until we return from the func 

	const query = `
		INSERT INTO attempts (execution_id, worker_id, attempt_num, state)
		SELECT $1, $2, COALESCE(MAX(attempt_num), 0) + 1, $3
		FROM attempts
		WHERE execution_id = $1
		RETURNING attempt_id, execution_id, worker_id, attempt_num, state, created_at
	`

	row := poll.QueryRow(ctx, query, executionID, workerID, AttemptStateCreated)

	var attempt models.Attempt
	if err := row.Scan(&attempt.AttemptID, &attempt.ExecutionID, &attempt.WorkerID, &attempt.AttemptNum, &attempt.State, &attempt.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert attempt: %w", err)
	}
	
	return &attempt, nil
}  // allocates attempt_num


// FinishAttempt marks an Attempt as finished, recording its final state and error.
func FinishAttempt(ctx context.Context, attemptID int64, state string, attemptErr error) error {
	poll, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer poll.Release()

	var errMsg *string
	if attemptErr != nil {
		msg := attemptErr.Error()
		errMsg = &msg
	}

	const query = `
		UPDATE attempts
		SET state = $1, error = $2, finished_at = NOW()
		WHERE attempt_id = $3
	`

	if _, err := poll.Exec(ctx, query, state, errMsg, attemptID); err != nil {
		return fmt.Errorf("finish attempt: %w", err)
	}
	return nil
}

// AppendEvent records one audit event for an Execution.
func AppendEvent(ctx context.Context, executionID int64, eventType string, payload any) error {
	poll, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer poll.Release()

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event payload: %w", err)
	}

	const query = `
		INSERT INTO events (execution_id, event_type, payload)
		VALUES ($1, $2, $3)
	`

	if _, err := poll.Exec(ctx, query, executionID, eventType, payloadJSON); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// WorkerStateIdle is the state a newly registered Worker starts in.
const WorkerStateIdle = "IDLE"

// RegisterWorker inserts a new Worker row and returns its id.
func RegisterWorker(ctx context.Context, taskQueueName string) (int64, error) {
	if pool == nil {
		return 0, fmt.Errorf("db.Init must be called before RegisterWorker")
	}

	const query = `
		INSERT INTO workers (current_state, task_queue_name)
		VALUES ($1, $2)
		RETURNING worker_id
	`

	var workerID int64
	if err := pool.QueryRow(ctx, query, WorkerStateIdle, taskQueueName).Scan(&workerID); err != nil {
		return 0, fmt.Errorf("insert worker: %w", err)
	}

	return workerID, nil
}

// HeartbeatWorker updates a Worker's current_state.
func HeartbeatWorker(ctx context.Context, workerID int64, state string) error {
	poll, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer poll.Release()

	const query = `
		UPDATE workers
		SET current_state = $1
		WHERE worker_id = $2
	`

	if _, err := poll.Exec(ctx, query, state, workerID); err != nil {
		return fmt.Errorf("update worker state: %w", err)
	}
	return nil
}
