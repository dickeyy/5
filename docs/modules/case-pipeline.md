# Case Pipeline

The case pipeline connects templates, permission-checked case creation, action
row generation, and later queued execution. The creation logic starts in
`internal/quack/cases.go`, while persistence lives in `internal/store/cases.go`.

## Inputs And Boundaries

Case creation is exposed through two entrypoints:

- HTTP: `POST /guilds/:discordGuildID/cases` in `internal/httpapi/routes/cases.go`
- Discord: `/case add` in `internal/discordbot/commands/case.go`

Both entrypoints eventually call `CaseService.Create(...)` through
`quack.Services`.

Staff dashboard case reads are exposed through the HTTP API:

- `GET /guilds/:discordGuildID/cases`
- `GET /guilds/:discordGuildID/cases/:caseRef`
- `GET /guilds/:discordGuildID/users/:targetDiscordUserID/cases`

These routes require the same foundation staff permission used for case
creation. `caseRef` resolves to a per-guild case number when it is numeric and
to a case ULID otherwise. List routes use offset pagination and return newest
cases first.

## Creation Flow

1. Validate guild context, permission, template ID, target user, source, and
   metadata.
2. Load the expanded template with `GetCaseTemplateExpanded`.
3. Reject archived templates.
4. Copy the immutable official reason from the template `reason_template`.
5. Choose the selected template level based on prior case count for the same
   target and template.
6. Build `TemplateSnapshotJSON` so the case keeps the policy that was used at
   creation time.
7. Build action execution rows from the selected level actions.
8. Add a generated `send_dm` execution first when the level itself has
   `notify_user` enabled.
9. Persist the case, initial case event, action executions, and audit row in
   one transaction.
10. Enqueue the case for asynchronous action processing.

## Level Selection Rules

Level selection in `selectTemplateLevel` depends on `is_default` and the
distinct positive `trigger_case_count` values of non-default levels.

The service counts prior non-voided cases for the same guild, template, and
target user across all time. Matching uses the current stored case history, not
transient queue state.

The default level acts as fallback when no escalation level matches. Validation
in `internal/quack/templates.go` expects exactly one default level.

## Snapshot Contract

`TemplateSnapshotJSON` is not just audit data. Runtime behavior depends on it.

Current consumers include API and Discord responses that expose selected level
and action details.

Changing the snapshot shape needs migration discipline because old cases keep
their stored JSON.

## Dashboard Read Contract

Case list responses include the selected level snapshot and current action
summary for each case. Case detail responses include the case fields, template
snapshot, action executions, action attempts grouped under each execution, and
timeline events ordered oldest first.

Target user history is the case list scoped to one Discord user plus summary
counts by validity and template. Audit log reads are exposed separately through
`GET /guilds/:discordGuildID/audit-log`, require `audit.read`, and support
offset pagination plus filters for actor, action, resource, and result.

## Action Execution Rows

For each selected template action, `CaseService.Create` snapshots:

- action type
- typed timeout duration or ban history-deletion settings
- safe automatic retry count

Storage then fills in:

- ULIDs
- `case_id`
- default pending status
- idempotency key in the form `case:<caseID>:action:<position>` when none is supplied
- internal retry timing and irreversible-action classification

`storage.CreateCase` also assigns the next per-guild case number inside the
transaction.

## Case Validity

Every live case has exactly one validity value: `valid` or `voided`. New cases
start valid. Action execution state remains on `case_action_executions`, and
appeal state remains on appeal records; neither changes case validity. The
future void workflow is the only product operation that may set a case to
voided.

Migration 0003 maps the former mixed status values to validity while retaining
the exact previous values in reversible migration-owned bookkeeping. Retired
note and generic status-change events remain stored for compatibility but are
excluded from live v5 event reads.

## Maintainability Notes

- Warning notification is modeled as a generated internal action, not as a
  direct side effect of case creation.
- The template snapshot is the durable policy source for a created case; live
  template edits do not rewrite old cases.
- Execution timeout, retry timing, and idempotency remain Quack-owned internals;
  they are not template input or snapshot fields.

Relevant files:

- `internal/quack/cases.go`
- `internal/quack/audit.go`
- `internal/quack/templates.go`
- `internal/httpapi/routes/cases.go`
- `internal/discordbot/commands/case.go`
- `internal/store/cases.go`
- `internal/store/templates.go`
- `internal/quack/model/schema.go`
