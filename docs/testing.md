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
transaction isolation uses a real MySQL-backed integration test when
`QUACK_TEST_MYSQL_DSN` is configured. Migration integration tests create and
drop their own uniquely named database rather than altering the database named
by that DSN.

## Current Coverage Pressure Points

The current backend has a few areas where targeted tests matter more than broad
end-to-end setup:

- template validation and level normalization in `internal/quack/templates.go`
- case level selection and snapshotting in `internal/quack/cases.go`
- action retries, notifications, and failure handling in `internal/quack/actions.go`
- route auth and guild-context wiring in `internal/httpapi/routes/router_test.go`
- interaction dispatch, deferred responses, and component/modal routing in
  `internal/discordbot/interactions/dispatcher_test.go`
- Discord response helper output shape in `internal/discordbot/ui/responses_test.go`

Relevant files:

- `internal/testutil/storage.go`
- `internal/testutil/config.go`
- `internal/httpapi/routes/router_test.go`
- `internal/quack/templates_test.go`
- `internal/quack/cases_test.go`
- `internal/quack/actions_test.go`
- `internal/discordbot/interactions/dispatcher_test.go`
- `internal/discordbot/ui/responses_test.go`
- `internal/store/migrations_test.go`
