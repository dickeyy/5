# Testing

## Current Test Harness

The current test suite is driven through `go test ./...`.

Many storage and route tests use in-memory SQLite through GORM and, when Redis
behavior matters, a `miniredis` server through `internal/testutil/storage.go`.
This keeps most tests fast and self-contained.

`internal/testutil/config.go` also installs a minimal test config so auth and
cookie-dependent code can run without the full production environment.

## Scope Limits

SQLite-backed tests are intended to catch:

- schema model registration
- migration execution
- repository wiring
- route behavior
- case/template business rules
- action execution control flow

They are not a replacement for MySQL coverage. Any behavior that depends on
MySQL-specific SQL, indexes, locking, JSON semantics, unsigned integers, or
transaction isolation should be covered later with a real MySQL-backed
integration test.

## Current Coverage Pressure Points

The current backend has a few areas where targeted tests matter more than broad
end-to-end setup:

- template validation and level normalization in `app/templates.go`
- case level selection and snapshotting in `app/cases.go`
- action retries, notifications, and failure handling in `app/actions.go`
- route auth and guild-context wiring in `api/routes/router_test.go`
- interaction dispatch, deferred responses, and component/modal routing in
  `discord/interactions/dispatcher_test.go`
- Discord response helper output shape in `discord/ui/responses_test.go`

Relevant files:

- `internal/testutil/storage.go`
- `internal/testutil/config.go`
- `api/routes/router_test.go`
- `app/templates_test.go`
- `app/cases_test.go`
- `app/actions_test.go`
- `discord/interactions/dispatcher_test.go`
- `discord/ui/responses_test.go`
- `storage/migrations_test.go`
