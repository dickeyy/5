# Audit, statistics, and Discord moderator experience

QP-E implements the v5 audit/statistics boundary and Discord case workflows.
The audit log remains distinct from the optional general-logging module.

## Audit contract

`model.AuditAction` is the stable action vocabulary. `model.AuditContract`
defines the resource type and whether an event is important enough for the
optional staff-channel mirror. Adapter source travels through
`quack.ContextWithAuditSource`; case sources map to dashboard, Discord,
honeypot, and v4-import audit sources without relying on caller metadata.

Audit metadata is a JSON object of bounded identifiers, states, counts, and
classifications. The storage boundary recursively redacts credential,
session, webhook, transport-payload, transcript, and member-content keys and
redacts credential-shaped failure text. GORM update and delete callbacks reject
normal application mutation of `audit_log_entries`; corrections append a new
entry.

All moderators may filter the guild log by actor, source, action, resource,
result, case, member, and RFC3339 date. Results order by `(created_at, id)`
descending and expose an immutable-entry cursor through `before_id` and
`next_cursor`. Permission-sensitive reads append their own trace-linked audit
entry without copying returned data into metadata.

## Audit mirror

`quack.AuditMirrorWorker` polls important immutable entries outside the
originating moderation transaction. Delivery therefore cannot block or roll
back moderation. Successful, failed, disabled, and repaired outcomes append
audit history referencing the original audit entry. A Discord 403/404 clears
the stale configured channel through the existing settings repair boundary.

`discordbot.Bot.SendAuditMirror` renders a bounded staff embed with mentions
disabled. It does not import, enqueue, or call optional general-logging code.

## Derived statistics

`quack.StaffStatisticsService` queries cases, case actions, appeals, and audit
entries directly for one guild and one bounded time range. Responses include
daily, template, validity, case-source, action-type/result, appeal-status, and
audit action/result/source breakdowns. No aggregate table, actor ranking, or
leaderboard is created.

## Discord moderator workflows

`commands.RegisterCaseComponents` installs real case list/user/failure
pagination, retry, dismissal, void-reason modal, confirmed reversal, message
template selection, and structured context handlers. Templates with more than
five context fields use a short-lived actor/guild-bound modal wizard. Public
creation results contain only case number, target, template, level, and action
status; validation remains private. The public status follow-up is edited when
normal asynchronous enforcement reaches a terminal result, while authorized
case views remain the recovery source of truth.

QI-2 must:

- call `routes.RegisterAuditStatisticsStaffRoutes` on the authenticated guild
  group;
- call `commands.RegisterCaseComponents` on the central component registry;
- call QP-D's separate `discordbot.RegisterAppealComponents` registrar;
- run `quack.NewAuditMirrorWorker(store, discordBot, interval).Run(ctx)` under
  the shared runtime lifecycle.

QP-E requires no schema migration or central migration registration.
