// Package temporal holds the names shared between the workflow definitions
// and anything that starts them, so a typo is a compile error rather than a
// workflow that silently never runs.
package temporal

const (
	// AgentExecutionWorkflowName is the registered name of the workflow.
	AgentExecutionWorkflowName = "agent-execution"

	// DefaultTaskQueue is used when TEMPORAL_TASK_QUEUE is unset.
	DefaultTaskQueue = "conductor"

	// DefaultAddress is used when TEMPORAL_ADDRESS is unset.
	DefaultAddress = "localhost:7233"

	// DefaultNamespace is used when TEMPORAL_NAMESPACE is unset.
	DefaultNamespace = "default"
)
