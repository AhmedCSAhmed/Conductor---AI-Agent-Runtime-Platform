# Conductor

A durable runtime for long-running AI agent executions.

## The Goal

When an AI agent runs for minutes or hours, everything that can go wrong will: the
process dies mid-run, a tool call times out, the model returns garbage on attempt one
and succeeds on attempt two. Conductor is the layer that makes those runs survivable.

Written in Go. An agent execution is submitted through an HTTP control plane, driven to
completion by a Temporal workflow, and mirrored into Postgres as a queryable record. Temporal owns
durability and retries. Postgres owns history, so you can answer "what happened on
attempt 2 of execution 41, and why did it fail" long after the workflow has closed.

The target experience: `POST /executions`, get an id back immediately, and watch a
full attempt-by-attempt timeline while the agent keeps working through crashes,
restarts, and deploys.

## Architecture

```
   client
     |
     | POST /executions            GET /executions/{id}
     v
 +--------------------------------------------------+
 |  HTTP control plane   (internal/api)              |
 |  validate -> start workflow -> return execution   |
 +--------------------------------------------------+
     |                                       ^
     | start_workflow                        | read state
     v                                       |
 +------------------------+                  |
 |  Temporal server       |                  |
 |  durable event history |                  |
 |  retries, timeouts     |                  |
 +------------------------+                  |
     |  task queue "conductor"               |
     v                                       |
 +--------------------------------------------------+
 |  Worker  (cmd/worker)                             |
 |                                                   |
 |   AgentExecutionWorkflow                          |
 |     |                                             |
 |     +--> activity: AddExecution                   |
 |     +--> activity: RunAgentStep     [planned]     |
 |     +--> activity: RecordAttempt    [planned]     |
 |     +--> activity: Finalize         [planned]     |
 +--------------------------------------------------+
     |  writes through internal/db/operations.go
     v                                       |
 +--------------------------------------------------+
 |  PostgreSQL   (internal/models/models.go)         |
 |                                                   |
 |   workers ---< attempts >--- executions ---< events
 |                                                   |
 |   worker     : pool member, state, task queue     |
 |   execution  : one agent run, current state       |
 |   attempt    : one try at an execution, error     |
 |   event      : append-only audit trail (JSON)     |
 +--------------------------------------------------+
```

## Where It Stands

Working today:

- Schema for `workers`, `executions`, `attempts`, `events`, created by `cmd/initdb`
- pgx connection pool, plain SQL, no ORM
- `AgentExecutionWorkflow` with a retry policy, calling a single `AddExecution` activity
- Worker binary that dials Temporal and polls the `conductor` task queue

Not there yet: `internal/api` has no routes and there is no `cmd/api`, so the only way
to start a workflow right now is the Temporal CLI or a hand-written Go program. There is
one write operation and no reads. Nothing yet runs an actual agent.

Every gap below is also marked as a numbered `TODO for myself` at the line in the code
where it bites. `grep -rn "TODO for myself" .` gives the full list.

## Next Weekend

Ordered so each step is testable before the next one starts.

0. **Add `workflow_id` and `run_id` to `Execution`, and make `AddExecution`
   idempotent.** Do this first. Nothing currently links a row back to the workflow
   that owns it, so cancel and reconcile are impossible, and the activity is a bare
   `INSERT` sitting under a 3-attempt retry policy, so a crash between commit and ack
   duplicates the row. Both fixes are one schema change and one `ON CONFLICT` clause,
   and everything below is built on that shape.

1. **Finish the repository layer.** `internal/db/operations.go` has exactly one
   function. Add `GetExecution`, `ListExecutions`, `UpdateExecutionState`, plus
   `CreateAttempt`, `FinishAttempt`, and `AppendEvent`. Move them onto a `Store`
   struct holding the pool instead of the package-level `pool` global, so a test can
   hand in its own transaction and roll it back.

2. **Stand up the control plane.** Create `cmd/api/main.go` holding the Temporal
   client and the pool, then wire routes on a stdlib `http.ServeMux` (Go 1.22+ has
   method and path patterns, so no router dependency):
   - `POST /executions` starts `AgentExecutionWorkflow` and returns the id
   - `GET /executions/{id}` returns state plus attempts
   - `GET /executions/{id}/events` returns the timeline
   - `POST /executions/{id}/cancel` signals the workflow, needs step 0
   - `GET /healthz` checks database and Temporal connectivity

   Set `ReadTimeout`, `WriteTimeout`, and graceful shutdown on `SIGTERM`. Go's
   zero-value `http.Server` has none of them.

3. **Make the workflow do real work.** Replace the single-activity workflow with a
   loop: mark RUNNING, create an attempt row, call the agent step, record success or
   the error, and finalize. Add `workflow.GetSignalChannel` for cancellation and
   `workflow.SetQueryHandler` for live status, so `GET /executions/{id}` can query the
   running workflow rather than polling the table.

4. **Wire up the agent step.** Add an activity that calls the Claude API, with
   `activity.RecordHeartbeat` and a `HeartbeatTimeout`, so a stalled model call is
   detected instead of hanging until `StartToCloseTimeout`.

5. **Cover it with tests.** There are none. `testsuite.WorkflowTestSuite` skips timers,
   so a workflow with a 30 second timeout tests instantly; `env.OnActivity` covers the
   retry path; `httptest` covers the routes once they exist.

Stretch, if the above lands early: replace `db.InitSchema` with real migration files
(golang-migrate or goose), since `CREATE TABLE IF NOT EXISTS` will never alter an
existing table and every step above changes the schema.

## Requirements

- Go 1.26+ (a transitive dependency of the Temporal SDK requires it; the toolchain
  downloads itself on first build)
- PostgreSQL 14+
- A Temporal server. For local development: `temporal server start-dev`

## Setup

```bash
go mod download
go run ./cmd/initdb
```

## Running

```bash
go run ./cmd/worker      # worker, polls the task queue
go run ./cmd/api         # control plane, once step 2 lands

go build ./...
go vet ./...
go test ./...
```

## Configuration

`.env` in the project root:

```
DATABASE_URL=postgres://postgres:postgres@localhost:5432/conductor
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE=conductor
```

A `DATABASE_URL` left over from the Python version (`postgresql+asyncpg://...`) still
works; `internal/db` strips the SQLAlchemy driver suffix, since pgx does not understand
it.

## Layout

```
cmd/worker/       worker binary, registers workflows and activities
cmd/initdb/       schema bootstrap
cmd/api/          control plane binary (not written yet)
internal/api/     HTTP handlers, request and response types
internal/models/  database row structs
internal/services/  business logic, independent of transport
internal/temporal/  workflow and activity definitions, shared names
internal/db/      pool, schema, repository functions
```

`internal/` is deliberate: nothing outside this module can import these packages, so
the layout stays free to change.
