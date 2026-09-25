// Package services holds the business logic that both the HTTP handlers and
// the Temporal activities call, so neither one talks to internal/db directly
// and the logic stays testable without a transport.
//
// TODO for myself: this package is empty. First thing to land here is an
// execution service wrapping start, read, and cancel, so POST /executions and
// the workflow share one code path instead of each writing their own.
//
// Define the interfaces this package needs (a small store interface, a small
// Temporal client interface) here rather than in the packages that implement
// them. That is the Go convention and it means tests can use fakes without
// touching Postgres.
package services
