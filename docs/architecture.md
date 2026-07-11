# Architecture

## Runtime

Quack is one Go process started by `cmd/quack/main.go`. The composition root in
`internal/runtime` loads immutable configuration, opens MySQL and Redis,
migrates the schema, creates the Discord adapter, application services, durable
work scheduler, and HTTP adapter, then shuts them down in reverse order.

Runtime dependencies are constructor-injected. There are no process-global
configuration, database, Redis, queue, Discord session, command, or event
registries.

## Boundaries

- `internal/quack` contains transport-independent use cases, domain models,
  repository ports, Discord gateway ports, and queue ports.
- `internal/store` implements repository ports with GORM/MySQL and Redis.
  GORM-tagged migration records remain private to this adapter.
- `internal/httpapi` translates HTTP requests, authentication, cookies, and
  response DTOs into application calls.
- `internal/discordbot` translates Discord interactions into the same
  application calls and owns Discord response rendering and command sync.
- `internal/workqueue` schedules persisted case actions for application
  processing.
- `internal/config` owns environment parsing without mutable global state.

Dependencies point inward: adapters may import the application core, while the
core imports no Gin, DiscordGo, GORM, or Redis packages.

## Moderation flow

Both `POST /guilds/:discordGuildID/cases` and Discord `/case add` call the
same case service. Case creation:

1. Resolves the actor through Discord-derived guild permissions.
2. Locks the guild row inside a unit of work.
3. Loads the selected template and counts matching non-voided history.
4. Selects and snapshots the highest matching escalation level.
5. Allocates the guild case number and writes the case, initial event, action
   executions, and audit row transactionally.
6. Submits a best-effort wake-up hint to the work queue.

The guild lock makes simultaneous cases observe a deterministic history and
receive unique case numbers.

## Action scheduling

Persisted `case_action_executions` rows are the source of truth. Immediate
submission reduces latency, but queue saturation cannot lose the action.

The scheduler discovers due pending or retrying cases at startup and every
second in batches of 100. Workers rely on database claim locks to prevent
duplicate execution. Retry timing is represented only by `next_retry_at`; no
per-retry goroutine is required.

Shutdown stops new submissions and polling, closes the job channel outside the
queue mutex, and drains workers.

Current action capability remains unchanged:

- `send_dm`: implemented.
- `timeout_user`, `kick_user`, and `ban_user`: recorded as unsupported
  failures.

## Persistence compatibility

Store-owned schema records preserve the existing table names, columns, indexes,
and JSON columns. Production startup runs an ordered, checksum-tracked migration
registry under a MySQL advisory lock; it does not call `AutoMigrate`. The
initial additive migration adopts a current pre-ledger v5 database without
rewriting its records. Plain domain models do not contain GORM tags. Redis
authentication and Discord command-cache key formats are unchanged.

## Delivery adapters

The HTTP and Discord adapters call `quack.Services` directly in-process.
Discord does not call the HTTP server. Existing HTTP routes, response fields,
trace headers, cookies, and Discord `/case add` behavior remain compatibility
contracts.

`Legacy/` is a separate Go module and behavioral reference; it is not part of
the v5 runtime dependency graph.
