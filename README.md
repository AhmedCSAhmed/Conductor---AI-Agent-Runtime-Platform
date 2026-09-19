# Conductor

A durable runtime for long-running AI agent executions.

## The Goal

When an AI agent runs for minutes or hours, everything that can go wrong will: the
process dies mid-run, a tool call times out, the model returns garbage on attempt one
and succeeds on attempt two. Conductor is the layer that makes those runs survivable.

An agent execution is submitted through an HTTP control plane, driven to completion by
a Temporal workflow, and mirrored into Postgres as a queryable record. Temporal owns
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
 |  FastAPI control plane   (app/api)                |
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
 |  Worker  (app/temporal/worker.py)                 |
 |                                                   |
 |   AgentExecutionWorkflow                          |
 |     |                                             |
 |     +--> activity: add_execution                  |
 |     +--> activity: run_agent_step   [planned]     |
 |     +--> activity: record_attempt   [planned]     |
 |     +--> activity: finalize         [planned]     |
 +--------------------------------------------------+
     |  writes through app/db/operations.py
     v                                       |
 +--------------------------------------------------+
 |  PostgreSQL   (app/models/models.py)              |
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

- Schema for `workers`, `executions`, `attempts`, `events`, created via `init_db.py`
- Async SQLAlchemy engine and session factory over asyncpg
- `AgentExecutionWorkflow` with a retry policy, calling a single `add_execution` activity
- Worker entrypoint that connects and polls the `conductor` task queue

Not there yet: the API surface is empty, so the only way to start a workflow right now
is from a Python shell. There is one write operation and no reads. Nothing yet runs an
actual agent.

## Next Weekend

Ordered so each step is testable before the next one starts.

1. **Finish the repository layer.** `app/db/operations.py` has exactly one function.
   Add `get_execution`, `list_executions`, `update_execution_state`, plus
   `create_attempt`, `finish_attempt`, and `append_event`. Take an `AsyncSession`
   as an argument instead of using the module-level `_db` singleton, so tests can
   inject a transaction and roll it back.

2. **Stand up the control plane.** `app/api/routes.py` is empty and `app/main.py`
   does not exist, even though the run instructions reference `app.main:app`. Create
   the FastAPI app with a lifespan that holds the Temporal client, then wire:
   - `POST /executions` starts `AgentExecutionWorkflow` and returns the id
   - `GET /executions/{id}` returns state plus attempts
   - `GET /executions/{id}/events` returns the timeline
   - `GET /healthz` checks database and Temporal connectivity

3. **Make the workflow do real work.** Replace the single-activity workflow with a
   loop: mark RUNNING, create an attempt row, call the agent step, record success or
   the error, and finalize. Add a signal for cancellation and a query for live status,
   so `GET /executions/{id}` can read from the workflow rather than polling the table.

4. **Wire up the agent step.** Add an activity that calls the Claude API through the
   Anthropic SDK with a heartbeat, so a stalled model call is detected instead of
   hanging until the timeout.

5. **Cover it with tests.** `tests/` is empty. Use Temporal's `WorkflowEnvironment`
   time-skipping harness for the workflow, mock activities for the retry path, and
   httpx `ASGITransport` for the routes.

Stretch, if the above lands early: swap `init_db.create_all` for a real Alembic
baseline revision, since `migrations/` currently holds only a `.gitkeep`.

## Requirements

- Python 3.12+
- PostgreSQL 14+
- A Temporal server. For local development: `temporal server start-dev`

## Setup

```bash
python3.12 -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
python -m app.db.init_db
```

## Running

```bash
source ~/Desktop/conductor/.venv/bin/activate

python -m app.temporal.worker      # worker, polls the task queue
uvicorn app.main:app --reload      # control plane, once step 2 lands
pytest
```

## Configuration

`.env` in the project root:

```
DATABASE_URL=postgresql+asyncpg://postgres:postgres@localhost:5432/conductor
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE=conductor
```

## Layout

```
app/api/       FastAPI routers, request and response schemas
app/models/    SQLAlchemy ORM models
app/services/  business logic, independent of transport
app/temporal/  workflows, activities, worker entrypoint
app/db/        engine, sessions, repository functions
migrations/    Alembic revisions
tests/
```
