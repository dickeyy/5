# Architecture

## Runtime Overview

Quack v5 currently runs as one Go process that starts the database connection,
Redis client, schema migrations, in-process event queue, Discord session, slash
commands, interaction dispatcher, and the HTTP API. The startup path is
implemented in `main.go`.

Startup order:

1. Load config from `.env` and process environment variables in `lib/config.go`.
2. Connect to MySQL through GORM in `services/db.go`.
3. Connect to Redis in `services/redis.go`.
4. Run schema migrations in `storage/migrations.go`.
5. Initialize and start the in-process event queue in `services/event-queue.go`.
6. Connect the Discord session in `discord/discord.go`.
7. Build app services in `app/app.go`.
8. Register slash commands and install the interaction dispatcher in
   `discord/commands/registry.go`.
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

## Container Packaging

The repository now includes first-party local container packaging:

- `compose.yaml` defines MySQL and Redis for the normal local workflow.
- The optional `app` profile builds the Go service from `Dockerfile`.
- The `app` service waits for healthy MySQL and Redis containers before it
  starts.

This packaging is for local runtime convenience and smoke testing. The core
runtime architecture is still the single Go process described above.

Relevant files:

- `compose.yaml`
- `Dockerfile`
- `.env.example`

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
- `GET /ops/status`
- `GET /auth/discord/login`
- `GET /auth/discord/callback`
- `GET /auth/me`
- `POST /auth/logout`
- `GET /guilds`
- `GET /guilds/:discordGuildID/me`
- Template CRUD under `/guilds/:discordGuildID/templates`
- `GET /guilds/:discordGuildID/cases`
- `POST /guilds/:discordGuildID/cases`
- `GET /guilds/:discordGuildID/cases/:caseRef`
- `GET /guilds/:discordGuildID/users/:targetDiscordUserID/cases`
- `GET /guilds/:discordGuildID/audit-log`
- `GET /guilds/:discordGuildID/ops/status`

Auth sessions are loaded by `api/middleware/auth.go`. Guild-scoped access is
resolved by `api/middleware/guild.go`, which builds `GuildStaffContext` through
the app layer and checks permission actions before handlers run.
Request and correlation IDs are installed by `api/middleware/request.go` and
are echoed through response headers, logs, case records, queued action events,
and audit rows.

`GET /ops/status` is guarded by `X-Quack-Ops-Key` plus `OPS_STATUS_TOKEN`.
Guild-scoped ops status also allows Discord guild owners and Administrators.

Relevant files:

- `api/server.go`
- `api/routes/router.go`
- `api/routes/ops.go`
- `api/routes/auth.go`
- `api/routes/guilds.go`
- `api/routes/templates.go`
- `api/routes/cases.go`
- `app/ops.go`
- `api/middleware/auth.go`
- `api/middleware/guild.go`
- `api/middleware/request.go`

## Discord Surface

Discord is currently both an operator surface and an execution surface.
Commands are registered during startup, and `/case add` is implemented against
the same application services used by the API. See `discord/commands/case.go`.

The current Discord stack has three layers:

- `discord/commands/`: command definitions, command sync, and command lookup
- `discord/interactions/`: runtime dispatch for commands, autocomplete,
  components, and modal submits
- `discord/ui/`: message, edit, response, embed, and custom-ID helpers

`commands.Register(...)` wires the dispatcher into the Discord session through
`interactions.NewDispatcher(...)`. Incoming application commands are resolved by
name through the command registry. Components and modals are resolved through
`ComponentRegistry` using decoded custom IDs.

Handler execution has two paths:

- immediate responses return a `discordgo.InteractionResponse` directly
- async responses first defer, then run a task that edits the original
  interaction response through the responder abstraction

The dispatcher also recovers from panics and converts task failures into a
standard error edit, which keeps the transport layer consistent across
commands.

Component and modal infrastructure exists now, but current production
registration is still command-centric. `/case` remains the only registered
command, and broad component/modal handler registration has not been added yet.

The working rule for current backend design is that Discord permissions are the
foundation for guild authorization, while templates and cases remain backend
records. The product-level rationale is described in `v5.md`.

Relevant files:

- `discord/discord.go`
- `discord/commands/registry.go`
- `discord/commands/case.go`
- `discord/interactions/dispatcher.go`
- `discord/interactions/components.go`
- `discord/ui/message.go`
- `discord/ui/responses.go`
- `discord/ui/views/case.go`
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

The event queue tracks accepted, dropped, processed, failed, and panicked event
counters. Queue events carry request and correlation IDs into action processing
so system audit rows can be tied back to the case creation request or Discord
interaction.

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
- `docs/modules/discord-interactions.md`

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
- `docs/modules/discord-interactions.md`
- `docs/modules/event-queue.md`
