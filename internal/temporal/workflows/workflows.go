// Package workflows holds the workflow definitions. Workflow code must be
// deterministic: no clocks, no random, no network, no goroutines outside the
// SDK's helpers. All of that belongs in an activity.
package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/AhmedCSAhmed/conductor/internal/temporal/workflows/activities"
)

// AgentExecutionWorkflow drives one agent run to completion.
//
// TODO for myself: this is a stub that creates one row. The real shape is a
// loop: mark RUNNING, create the attempt row, run the agent step, record
// success or the error, finalize. Plus workflow.GetSignalChannel for cancel
// and workflow.SetQueryHandler for live status, so GET /executions/{id} can
// query the running workflow instead of polling the table.
func AgentExecutionWorkflow(ctx workflow.Context, name string) (int64, error) {
	// TODO for myself (2): this retry policy sits on a non-idempotent insert.
	// Fix the activity before trusting the retry.
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	// TODO for myself (1): pass workflow.GetInfo(ctx).WorkflowExecution.ID and
	// .RunID into the activity so the Execution row can point back here.
	var executionID int64
	err := workflow.ExecuteActivity(ctx, activities.AddExecution, name).Get(ctx, &executionID)
	if err != nil {
		return 0, err
	}

	return executionID, nil
}

// TODO for myself: still need more workflows, or more likely one workflow with
// real steps: the agent loop above, plus a child workflow per tool call once a
// single run gets long enough that one event history is too big.
