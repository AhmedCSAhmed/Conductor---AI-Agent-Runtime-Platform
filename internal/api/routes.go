// Package api is the HTTP control plane.
//
// TODO for myself: this package has no routes yet and there is no cmd/api, so
// there is no HTTP surface at all right now. The only way to start a workflow
// today is a hand-written Go program or the Temporal CLI.
//
// Build in this order:
//
//  1. cmd/api/main.go: build the Temporal client and the db pool once in main,
//     pass them into a Server struct, and give http.Server real
//     ReadTimeout/WriteTimeout plus graceful shutdown on SIGTERM. Go makes it
//     easy to forget all three.
//  2. Request and response types in this package, kept separate from
//     internal/models, which is the database layer.
//  3. Routes, on the stdlib http.ServeMux (Go 1.22+ handles method and path
//     patterns, so no router dependency is needed):
//     POST /executions              start AgentExecutionWorkflow, return id
//     GET  /executions              list, paginated
//     GET  /executions/{id}         state plus attempts
//     GET  /executions/{id}/events  the timeline
//     POST /executions/{id}/cancel  signal the workflow, needs the stored
//     workflow_id from TODO (1)
//     GET  /healthz                 check Postgres and Temporal connectivity
//
// Handlers should call internal/services, never internal/db directly.
package api
