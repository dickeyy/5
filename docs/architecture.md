# Architecture

## Runtime Overview

Quack v5 currently runs as one Go process that starts the database connection,
Redis client, schema migrations, in-process event queue, Discord session, slash
commands, and the HTTP API. The startup path is implemented in `main.go`.

Startup order:

1. Load config from `.env` and process environment variables in `lib/config.go`.
2. Connect to MySQL through GORM in `services/db.go`.
3. Connect to Redis in `services/redis.go`.
4. Run schema migrations in `storage/migrations.go`.
5. Initialize and start the in-process event queue in `services/event-queue.go`.
6. Connect the Discord session in `discord/discord.go`.
7. Build app services in `app/app.go`.
8. Register slash commands in `discord/commands/registry.go`.
9. Re-enqueue pending case actions with `app.EnqueuePendingCaseActions` in `app/actions_queue.go`.
10. Start the Gin API in `api/server.go`.

Relevant files:

- `main.go`
- `lib/config.go`
- `services/db.go`
- `services/redis.go`
- `services/event-queue.go`
- `storage/migrations.go`
- `app/app.go`
- `api/server.go`

## Layering

The codebase is split into a few clear layers:

- `api/`: HTTP server, middleware, and route handlers.
- `discord/`: Discord session wiring and command handlers.
- `app/`: application services and business rules shared across HTTP and Discord entrypoints.
- `storage/`: repository layer and migrations.
- `structs/`: shared domain models, enums, config structs, and schema definitions.
- `services/`: process-wide infrastructure clients such as DB, Redis, and the event queue.

`app.Services` is the service boundary shared by API routes and Discord code.
That assembly lives in `app/app.go`.

## HTTP Surface

The Gin router is created in `api/server.go`. CORS currently allows only
`http://localhost:3000` and `http://127.0.0.1:3000`, and cookie auth is enabled
for those origins.

`api/routes/router.go` wires the live routes:

- `GET /status`
- `GET /auth/discord/login`
- `GET /auth/discord/callback`
- `GET /auth/me`
- `POST /auth/logout`
- `GET /guilds`
- `GET /guilds/:discordGuildID/me`
- Template CRUD under `/guilds/:discordGuildID/templates`
- `POST /guilds/:discordGuildID/cases`

Auth sessions are loaded by `api/middleware/auth.go`. Guild-scoped access is
resolved by `api/middleware/guild.go`, which builds `GuildStaffContext` through
the app layer and checks permission actions before handlers run.

Relevant files:

- `api/server.go`
- `api/routes/router.go`
- `api/routes/auth.go`
- `api/routes/guilds.go`
- `api/routes/templates.go`
- `api/routes/cases.go`
- `api/middleware/auth.go`
- `api/middleware/guild.go`

## Discord Surface

Discord is currently both an operator surface and an execution surface.
Commands are registered during startup, and `/case add` is implemented against
the same application services used by the API. See `discord/commands/case.go`.

The working rule for current backend design is that Discord permissions are the
foundation for guild authorization, while templates and cases remain backend
records. The product-level rationale is described in `v5.md`.

Relevant files:

- `discord/discord.go`
- `discord/commands/registry.go`
- `discord/commands/case.go`
- `app/guilds.go`
- `v5.md`

## Templates, Levels, and Cases

Templates are stored as guild-owned moderation policies in `structs.CaseTemplate`
and expanded through storage helpers in `storage/templates.go`.

Current template behavior in the app layer:

- Templates must contain `levels`, and exactly one enabled default level is
  expected by validation in `app/templates.go`.
- Actions are authored under `levels[].actions`; there is no flat template
  action list or separate escalation-rule list in the app model.
- Creating a case is the warning. `record_warning` is not an action type and no
  execution row is created just to record a warning.
- Template-authored executable action types are currently `timeout_user`,
  `kick_user`, and `ban_user`.
- Warning notification is modeled as `notify_user` plus `notification_type` on
  the selected level. Moderation-action notification uses the same fields on
  the action. `send_dm` is an internal queued execution, not a template action.
- When `notify_user` is enabled, validation normalizes `notification_type` and
  defaults it from the moderation action type in `app/templates.go`.

Case creation in `app/cases.go` loads the selected template, resolves the
highest matching level by prior case count for the same template and target
user, snapshots the selected level and actions, creates action executions, and
writes the initial case event and audit log entry.

Relevant files:

- `app/templates.go`
- `app/cases.go`
- `storage/templates.go`
- `storage/cases.go`
- `structs/schema.go`

## Action Execution

Case actions are executed asynchronously.

Flow:

1. Case creation queues the case through `enqueueCaseActions` in `app/actions_queue.go`.
2. The in-process queue in `services/event-queue.go` runs handlers with a worker pool.
3. `ActionService.ProcessCaseActions` in `app/actions.go` claims the next
   executable action from storage.
4. The action handler runs, completion state is written back through
   `storage.CompleteCaseAction`, and later actions may be skipped on failure.
5. Retryable failures are rescheduled by writing `next_retry_at` and starting a
   delayed re-enqueue.

Current execution support splits into two layers:

- Template-authored actions can currently create case executions for
  `timeout_user`, `kick_user`, and `ban_user`.
- Warning-level notification creates an internal `send_dm` execution when the
  selected level has `notify_user` enabled.
- `timeout_user`, `kick_user`, and `ban_user` are recognized action modules, but
  their current implementations return `action_not_implemented` until the
  Discord moderation calls are filled in.

Notification behavior: `CaseTemplateLevel`, `CaseTemplateLevelAction`, and
`CaseActionExecution` persist notification metadata. Warning notification comes
from the selected level; moderation-action notification comes from the action.
This is part of the current schema applied by `storage/migrations.go`.

Relevant files:

- `app/actions.go`
- `app/actions_queue.go`
- `services/event-queue.go`
- `storage/cases.go`
- `storage/migrations.go`
- `structs/schema.go`

Module docs for this area:

- `docs/modules/action-engine.md`
- `docs/modules/event-queue.md`
- `docs/modules/case-pipeline.md`

## Data Model

The schema registry is defined in `structs/schema.go` and applied through
`storage/migrations.go`.

Important records:

- `Guild`, `StaffMember`
- `CaseTemplate`, `CaseTemplateLevel`, `CaseTemplateLevelAction`
- `Case`, `CaseActionExecution`, `CaseActionAttempt`, `CaseEvent`
- `Appeal`, `AppealEvent`
- `Ticket`, `TicketEvent`
- `AuditLogEntry`
- `AuthSession`, `OAuthState` for login/session state

The current migration set is:

- `0001_v5_schema`

Relevant files:

- `structs/schema.go`
- `storage/migrations.go`
- `storage/storage.go`

## Module Docs

For subsystem-level notes, use:

- `docs/modules/action-engine.md`
- `docs/modules/case-pipeline.md`
- `docs/modules/command-registry.md`
- `docs/modules/event-queue.md`
