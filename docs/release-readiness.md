# Release Readiness

Phase 8 prepares v5 for real-guild testing without adding appeals, tickets, or
v4 data import.

## Ops Status

Public dependency health remains available at:

- `GET /status`

Operational status is guarded:

- `GET /ops/status`
  - Requires `X-Quack-Ops-Key`.
  - The key must match `OPS_STATUS_TOKEN`.
  - If `OPS_STATUS_TOKEN` is unset, this endpoint returns disabled status.
- `GET /guilds/:discordGuildID/ops/status`
  - Allows `X-Quack-Ops-Key` when `OPS_STATUS_TOKEN` is set.
  - Also allows a normal Discord-authenticated session when the user is the
    Discord guild owner or has Discord `Administrator`.
  - Moderators with only `Moderate Members` are denied.

The ops payload includes queue counters, worker configuration, action execution
status counts, the oldest pending/retry action, recent action failures, and the
runtime action capability table.

Current action capability truth:

- `send_dm`: implemented.
- `timeout_user`: not implemented.
- `kick_user`: not implemented.
- `ban_user`: not implemented.

Punitive action rows are intentionally visible as failed
`action_not_implemented` executions until real Discord moderation calls are
implemented.

## Request Tracing

HTTP requests may send `X-Request-ID`. Quack validates the value and echoes it
back. Invalid or missing IDs are replaced with generated trace IDs.

Quack also tracks a correlation ID. API, Discord interaction, queue, case,
action, and audit paths carry these IDs so a single moderation flow can be
followed through logs and audit records.

## v4/v5 Coexistence

Run v4 and v5 as separate processes with separate configuration, command sync
settings, and storage. Do not point v5 at v4 tables.

Recommended rollout:

1. Run v5 against its own MySQL schema and Redis database.
2. Register v5 commands in a dev guild first with `DISCORD_COMMAND_GUILD_ID`.
3. Create a small set of v5 templates manually.
4. Test `/case add` and dashboard case creation against a private guild.
5. Keep v4 responsible for legacy appeals, tickets, and historical commands.
6. Enable v5 command registration for real guild testing only after the smoke
   checks below pass.

Rollback is command-level and process-level: stop v5, disable or prune v5
commands, and continue using v4. v5 does not import historical escalation
counts from v4 in this phase, so v5 escalation starts with v5-created cases.

## Release Checklist

- Required env vars are set: database, Redis, Discord token/app ID, OAuth client
  secret, OAuth callback, and queue sizing.
- `OPS_STATUS_TOKEN` is set only where Quack developer ops access is needed.
- `go test ./...` passes.
- `/status` reports database, Redis, and Discord connectivity.
- `/ops/status` works with the ops key and fails without it.
- A guild owner/Administrator can read guild ops status; a moderator cannot.
- Command sync targets the intended guild/global scope.
- A template can be created with a default level.
- API case creation and Discord `/case add` both create cases through the same
  template-driven flow.
- DM action execution succeeds in a controlled test.
- `timeout_user`, `kick_user`, and `ban_user` failures are visible in case
  detail, audit logs, and ops status as unsupported actions.

Relevant files:

- `internal/httpapi/middleware/request.go`
- `internal/httpapi/routes/ops.go`
- `internal/quack/actions.go`
- `internal/quack/ops.go`
- `lib/trace.go`
- `internal/store/ops.go`
