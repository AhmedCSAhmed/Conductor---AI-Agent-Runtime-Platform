// Command worker polls the Temporal task queue and runs workflows and
// activities. This is the process that must stay up for executions to advance.
package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/AhmedCSAhmed/conductor/internal/db"
	conductortemporal "github.com/AhmedCSAhmed/conductor/internal/temporal"
	"github.com/AhmedCSAhmed/conductor/internal/temporal/workflows"
	"github.com/AhmedCSAhmed/conductor/internal/temporal/workflows/activities"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("worker: %v", err)
	}
}

func run() error {
	// TODO for myself (11): second godotenv.Load in the codebase, the other is
	// in internal/db. Should read from a shared internal/config package.
	_ = godotenv.Load()

	ctx := context.Background()

	// The activities talk to Postgres, so the pool has to exist before the
	// worker starts accepting tasks.
	if err := db.Init(ctx); err != nil {
		return err
	}
	defer db.Close()

	temporalClient, err := client.Dial(client.Options{
		HostPort:  envOr("TEMPORAL_ADDRESS", conductortemporal.DefaultAddress),
		Namespace: envOr("TEMPORAL_NAMESPACE", conductortemporal.DefaultNamespace),
	})
	if err != nil {
		return err
	}
	defer temporalClient.Close()

	taskQueue := envOr("TEMPORAL_TASK_QUEUE", conductortemporal.DefaultTaskQueue)
	w := worker.New(temporalClient, taskQueue, worker.Options{})

	w.RegisterWorkflowWithOptions(
		workflows.AgentExecutionWorkflow,
		workflow.RegisterOptions{Name: conductortemporal.AgentExecutionWorkflowName},
	)
	w.RegisterActivity(activities.AddExecution)

	log.Printf("worker polling task queue %q", taskQueue)

	// Run blocks until the process is interrupted.
	return w.Run(worker.InterruptCh())
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
