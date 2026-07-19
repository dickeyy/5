# Case Pipeline

The case pipeline connects versioned templates, structured member-visible
context, immutable Discord evidence, live authorization, optional enforcement,
and one case-level notification. Core orchestration lives in
`apps/backend/internal/quack/cases.go`; atomic persistence lives in
`apps/backend/internal/store/cases.go`.

## Entrypoints

- HTTP case creation: `POST /guilds/:discordGuildID/cases`.
- Discord creation: `/case add` and the `Create moderation case` message
  context command.
- Staff reads: guild case search, detail, and member history.
- Member reads: the registrars in `core_moderation_routes.go` use the
  authenticated Discord identity, not current guild membership.

`RegisterCoreModerationStaffRoutes` exposes template restore/import/export,
void, failed-action, retry, dismissal, and reversal handlers without owning the
central router. `RegisterCoreModerationMemberRoutes` exposes target-owned reads.

## Atomic Creation Flow

1. Validate guild, template, target, source, metadata, context, and the optional
   idempotency key.
2. Load an active template and select the highest reached all-time threshold.
   Counts include the new case and only non-voided, non-v4-import cases for the
   same guild, target, and template identity across versions.
3. Refresh actor, bot, target, permissions, and both role hierarchies through
   Discord. Reject unsafe targets or an action either actor cannot perform.
4. Capture linked Discord messages before the database transaction. Enforce
   message, embed, attachment, and total-work bounds. Eligible attachments are
   copied to the managed staff-only evidence channel; a copy failure retains
   metadata and becomes a visible partial-capture warning.
5. Acquire the guild transaction lock and reselect the template version and
   escalation. A concurrent change causes a retry instead of committing stale
   preflight data.
6. Snapshot context definitions/values, official reason, template version,
   selected level, and its zero-or-one action.
7. Atomically assign the never-reused guild case number and persist the case,
   event, evidence, action work, notification work, and audits.
8. Submit the case ID as a queue wake-up hint after commit.

An idempotency key maps repeated Discord interactions or HTTP requests to the
same durable case and cannot be reused for a different target/template request.

## Reads And Privacy

Staff search supports case number, target, moderator, template, validity,
action result, appeal status, RFC3339 date bounds, stable newest-first ordering,
and bounded pagination. Detail includes snapshots, evidence, action attempts,
events, and notification state.

Member detail is available only to the target Discord identity. It includes the
official reason, visible context/evidence, validity and correction link,
selected outcome, public history, notification state, and appealability. It
hides moderator identity, raw Discord errors/payloads, worker IDs, retry state,
and staff-only evidence channel identity. Permission-sensitive reads are
audited.

## Correction

`CaseService.Void` requires a reason, preserves the case, appends public
history, removes it from escalation, and cancels work that has not crossed an
external-delivery boundary. A replacement is a new case with immutable links;
case numbers are never reused. Action and notification failure never changes
case validity.

## Durable Boundaries

- `TemplateSnapshotJSON` remains the understandable policy source for old
  cases after template edits.
- Evidence uses transport-neutral snapshots; Discord objects never enter core
  persistence.
- `notify_user` creates one `case_notifications` row, never a `send_dm` action.
- Retired compatibility columns and events remain stored but do not shape live
  v5 behavior.
