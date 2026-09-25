package workflows

// TODO for myself: zero tests so far. `go vet` runs as part of `go test`, but
// nothing enforces anything else yet.
//
// Worth setting up before the workflow gets complicated, not after:
//
//   - go.temporal.io/sdk/testsuite: WorkflowTestSuite gives a test environment
//     that skips timers, so a workflow with a 30 second timeout runs instantly
//     and deterministically.
//   - env.OnActivity(...) to mock AddExecution and cover the retry path,
//     especially the idempotency fix in TODO (2).
//   - httptest.NewServer against the handlers once internal/api exists.
//   - a test that opens a pgx transaction and rolls it back, which needs the
//     TODO (3) package-level pool removed first.
//   - a GitHub Actions workflow running `go vet ./...`, `go test ./...`, and
//     staticcheck.
