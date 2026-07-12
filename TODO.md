# Quack v5 Remaining Work

This checklist tracks unfinished work in this backend repository for the product defined in [`v5.md`](v5.md). It excludes implementation of the separate dashboard web application, but includes the backend contracts and behavior that the dashboard needs.

Items are grouped by concern, not by implementation order or priority. Completed infrastructure is intentionally omitted.

## Product Model Alignment

- [x] Remove escalation time windows from live domain models, template requests, responses, snapshots, validation, and tests while retaining frozen compatibility columns only.
- [x] Change escalation counting to use all-time non-voided cases only.
- [x] Remove severity from templates, cases, API contracts, live storage records, snapshots, filters, and tests while retaining frozen compatibility columns only.
- [x] Remove weight from cases, escalation behavior, live storage records, API contracts, and tests while retaining the frozen compatibility column only.
- [x] Remove the separate enabled state from live templates, levels, and level actions while retaining frozen compatibility columns only.
- [ ] Make reversible archive and restore the only template availability lifecycle.
- [ ] Remove soft-delete concepts that imply templates or cases can be deleted outside the defined archive/void flows.
- [x] Remove moderator reason overrides from case inputs, HTTP routes, Discord commands, snapshots, and tests.
- [x] Enforce zero or one timeout, kick, or ban action per template level.
- [x] Remove ordered multi-action behavior and `continue_on_error` from the product model.
- [x] Remove action-level notification settings now that each case has one level-owned notification decision.
- [x] Remove technical action settings that are no longer admin-facing, including configurable backoff, execution timeout, and idempotency scope.
- [x] Retain only the admin-configurable safe retry-count limit in template action contracts.
- [x] Replace mixed case lifecycle statuses with valid or voided case state.
- [x] Keep action execution and appeal statuses separate from case validity.
- [x] Remove note-related case event types and note-oriented model fields from the live v5 boundary while preserving retired rows for compatibility.
- [x] Remove free-form staff and member case-note expectations from backend contracts and documentation.
- [x] Restrict normal case sources to dashboard, Discord, honeypot automation, and v4 import where appropriate.
- [x] Preserve existing v5 data through explicit compatibility migrations while rejected fields are retired.
- [x] Update all current template and case JSON contracts and snapshots to match the simplified v5 model.
- [x] Update technical documentation after each model correction so it continues to describe running code.
- [x] Remove resolved entries from `docs/v5-scope-drift.md` only after code and tests match `v5.md`.

## Guild Setup and Settings

- [ ] Add guild settings records and repository methods for core Quack configuration.
- [ ] Add settings for the Discord audit-mirror channel.
- [ ] Add settings for the managed evidence channel.
- [ ] Add settings for the optional member-notification introduction and footer.
- [ ] Add settings for independently enabling tickets, general logging, and honeypots.
- [ ] Add backend read and write APIs for guild settings.
- [ ] Restrict guild settings changes to the owner, `Administrator`, or `Manage Guild` as defined by the setting.
- [ ] Audit successful, failed, and denied guild-setting changes.
- [ ] Bootstrap a guild when Quack is first installed instead of waiting for an arbitrary dashboard request.
- [ ] Create the active General rule violation starter template during guild bootstrap.
- [ ] Seed starter cases 1–2 as case-only notifications.
- [ ] Seed starter cases 3–4 with a 24-hour timeout.
- [ ] Seed starter case 5 and later with a ban and 24-hour message-history deletion.
- [ ] Make starter-template cases appealable and member notifications enabled at every level.
- [ ] Store whether the one-time starter-policy review notice has been shown.
- [ ] Expose the starter-policy notice through the backend for the dashboard setup flow.
- [ ] Create and permission the staff-only evidence channel during guild setup.
- [ ] Detect and report evidence-channel permission drift.
- [ ] Add a safe repair flow for a missing, deleted, or misconfigured evidence channel.
- [ ] Handle Discord guild create, update, leave, and rejoin events.
- [ ] Preserve guild history when Quack leaves instead of hard-deleting guild data.
- [ ] Define and implement reactivation behavior when Quack rejoins a known guild.
- [ ] Refresh stored guild name, icon, owner, and active state from Discord events.
- [ ] Clean or repair channel references when configured Discord channels are deleted.
- [ ] Document the Discord install permissions and privileged intents required by core and optional modules.
- [ ] Reduce hard-coded Discord intents to the minimum enabled feature set.

## Discord Identity and Permissions

- [ ] Refresh Discord guild membership and permission bits before every sensitive write or enforcement operation.
- [ ] Stop relying on session-time permission snapshots for sensitive authorization.
- [ ] Treat stored staff-member rows as audit/display caches only and never as an independent source of permission.
- [ ] End former staff access on their next protected request while preserving historical attribution.
- [ ] Grant full guild access to the guild owner and members with `Administrator`.
- [ ] Grant template and module configuration to members with `Manage Guild`.
- [ ] Grant case creation, case review, appeal review, audit reads, case voiding, and failure dismissal to members with `Moderate Members`.
- [ ] Allow all moderators to read the complete audit log instead of limiting it to `Manage Guild`.
- [ ] Require `Moderate Members` for selected timeout actions.
- [ ] Require `Kick Members` for selected kick actions.
- [ ] Require `Ban Members` for selected ban actions.
- [ ] Require the matching current Discord permission for every manual action retry or reversal.
- [ ] Check the moderator's role hierarchy against the target before punitive case creation.
- [ ] Check the Quack bot's role hierarchy against the target before punitive case creation.
- [ ] Check the Quack bot's action-specific Discord permissions before case creation.
- [ ] Block the entire case request when the selected level cannot be enforced by the actor or bot.
- [ ] Reject normal cases targeting the acting moderator.
- [ ] Reject normal cases targeting bot accounts, including Quack.
- [ ] Reject normal cases targeting the guild owner.
- [ ] Require normal template application targets to be current human guild members.
- [ ] Keep imported history and explicit reversal flows able to reference departed members without weakening normal target rules.
- [ ] List dashboard guild access for moderators with Quack capabilities instead of requiring `Manage Guild` for every staff entry point.
- [ ] Provide a member-facing guild/case entry path that does not depend on the banned user still appearing in Discord's guild list.
- [ ] Add consistent authorization errors for dashboard and Discord callers.
- [ ] Audit denied sensitive operations with actor, guild, requested capability, request ID, and correlation ID.
- [ ] Add permission-matrix tests covering owner, Administrator, Manage Guild, Moderate Members, Kick Members, Ban Members, former staff, and ordinary members.
- [ ] Add hierarchy tests covering moderator, bot, owner, peer-role, and higher-role targets.

## Templates and Escalation

- [ ] Add simple structured template context-field definitions.
- [ ] Support short-text context fields.
- [ ] Support long-text context fields.
- [ ] Support boolean context fields.
- [ ] Support number context fields.
- [ ] Support Discord-message-link context fields.
- [ ] Support required and optional context fields.
- [ ] Include context definitions in template create, update, get, list, import, export, and snapshot contracts.
- [ ] Validate context-field names, labels, types, ordering, and required flags.
- [ ] Treat all submitted context fields as member-visible.
- [x] Keep the official template reason fixed during case creation.
- [x] Require exactly one default level on every template.
- [x] Allow the default level to create a case without an enforcement action.
- [x] Validate positive case-count thresholds for every non-default level.
- [x] Reject duplicate or ambiguous threshold definitions.
- [x] Select the highest reached threshold and continue using it until a higher threshold is reached.
- [x] Count the new case when evaluating escalation.
- [x] Count matching non-voided cases across all versions of the same template identity.
- [ ] Exclude voided and imported-v4 historical cases from escalation.
- [x] Preserve immutable case snapshots when templates are edited.
- [x] Ensure template edits increment the version without creating a new escalation identity.
- [ ] Implement reversible template archive behavior.
- [ ] Implement template restore behavior.
- [ ] Keep archived templates readable but unavailable for new cases.
- [ ] Prevent archived templates from appearing in Discord autocomplete.
- [ ] Add template export with policy fields only.
- [ ] Exclude guild IDs, Discord channel IDs, history, audit data, and secrets from template exports.
- [ ] Add template import validation and confirmation.
- [ ] Create a new guild-owned template identity for every import.
- [ ] Make valid imported templates active immediately after confirmation.
- [x] Validate timeout duration as a required setting for timeout levels.
- [x] Validate Discord-supported message-history deletion values for ban levels.
- [x] Validate the safe retry-count limit.
- [ ] Audit template create, update, archive, restore, import, and export success and failure.
- [ ] Add backend contract tests for every template request and response shape.
- [ ] Add template-version and cross-version escalation tests.
- [ ] Add archive/restore and import/export round-trip tests.

## Case Creation and History

- [ ] Replace reason-override inputs with structured context submissions.
- [ ] Validate every required template context field before creating a case.
- [ ] Reject unknown, duplicate, or incorrectly typed context values.
- [ ] Snapshot submitted context with the case.
- [ ] Snapshot the selected level and its zero or one action with the case.
- [ ] Keep case creation, escalation selection, numbering, snapshot, event, action row, notification work record, and audit insertion atomic.
- [ ] Validate target membership, actor permission, bot permission, and both role hierarchies before committing the case.
- [ ] Keep case numbers unique, sequential per guild, and never reusable after voiding or import.
- [ ] Add case void service behavior with a required reason.
- [ ] Add case void storage behavior and immutable case event.
- [ ] Remove voided cases from future escalation counts without deleting their records.
- [ ] Add backend endpoints for voiding cases.
- [ ] Prevent edits that change a case target, template, selected level, reason, or action.
- [ ] Preserve an explicit replacement-case reference when staff recreate an incorrect case.
- [ ] Keep terminal action failures from automatically voiding the case.
- [ ] Separate action progress and appeal progress from case validity in all case responses.
- [ ] Add staff case search by case number, member, moderator, template, validity, action result, appeal status, and date.
- [ ] Add staff case pagination and stable sorting for large guilds.
- [ ] Add staff member-history summaries derived from valid and voided cases.
- [ ] Add member-owned case list and case-detail services.
- [ ] Authorize member case access by target Discord identity rather than current guild membership.
- [ ] Allow banned and departed members to read only cases targeting their Discord identity.
- [ ] Include valid and voided cases in member history with clear labels.
- [ ] Hide moderator identities from member-facing case responses.
- [ ] Hide raw Discord errors, worker IDs, internal retry fields, and technical action payloads from member-facing responses.
- [ ] Show official reason, visible context, evidence, selected outcome, public history, and appeal state to the member.
- [ ] Define and enforce public, staff, and internal case-event response views without reintroducing free-form notes.
- [ ] Audit permission-sensitive case and history reads.
- [ ] Add concurrency tests for simultaneous case creation and voiding in the same guild.
- [ ] Add authorization tests proving members cannot enumerate other users or guild records.

## Evidence Capture and Preservation

- [ ] Add domain models and storage records for message evidence snapshots.
- [ ] Add domain models and storage records for attachment metadata and preserved copies.
- [ ] Link evidence records to a case without embedding transport-specific Discord objects in the core.
- [ ] Add a shared evidence-capture service used by Discord and HTTP case creation.
- [ ] Parse and validate Discord message links.
- [ ] Verify that a pasted message belongs to the selected guild.
- [ ] Verify that Quack can access the linked channel and message.
- [ ] Fetch live message text, author, timestamps, IDs, embeds, and attachments.
- [ ] Snapshot message content before case creation commits.
- [ ] Add a Discord message context action for starting a case from a live message.
- [ ] Derive the target member from the selected message author in the context-action flow.
- [ ] Add template selection and structured-context collection after the message context action.
- [ ] Allow pasted message links in Discord case creation.
- [ ] Allow pasted message links through backend case-creation contracts used by the dashboard.
- [ ] Re-upload supported attachments to the managed staff-only evidence channel.
- [ ] Store copied evidence message and attachment references.
- [ ] Enforce Discord upload size and file-type limits without blocking unrelated case creation.
- [ ] Preserve filename, content type, size, and original URL when an attachment cannot be copied.
- [ ] Warn staff when attachment bytes were not preserved.
- [ ] Require explicit visible context when a linked message is already deleted or inaccessible.
- [ ] Prevent evidence capture from silently changing the case target.
- [ ] Present captured evidence to the affected member through an authorized backend response.
- [ ] Avoid exposing the staff-only evidence channel or unrelated evidence records to members.
- [ ] Handle expired Discord attachment URLs by resolving the managed evidence copy when available.
- [ ] Define evidence behavior when the managed channel is deleted between capture and read.
- [ ] Audit evidence capture success, partial capture, and failure.
- [ ] Add limits for message length, embed count, attachment count, and total capture work.
- [ ] Add tests for live messages, deleted messages, inaccessible channels, wrong-guild links, oversized files, unsupported files, and partial capture.

## Discord Enforcement Actions

- [ ] Expand the Discord action port beyond direct messages to timeout, kick, ban, timeout removal, and unban operations.
- [ ] Implement real `timeout_user` execution.
- [ ] Implement real `kick_user` execution.
- [ ] Implement real `ban_user` execution.
- [ ] Apply the template-defined timeout duration exactly.
- [ ] Apply the template-defined ban message-history deletion window exactly.
- [ ] Generate Discord audit-log reasons from the case number and official reason.
- [ ] Classify Discord validation, permission, hierarchy, unknown-member, rate-limit, server, timeout, and network failures.
- [ ] Mark only known-safe failures as automatically retryable.
- [ ] Never automatically retry uncertain irreversible kick or ban outcomes.
- [ ] Enforce the admin-configured retry-count limit for safe automatic retries.
- [ ] Keep retry timing and backoff internal to Quack.
- [ ] Add execution timeouts for outbound Discord calls.
- [ ] Add idempotency protection around every enforcement attempt.
- [ ] Prevent duplicate execution when the same case is submitted repeatedly.
- [ ] Prevent duplicate execution when Discord or the HTTP caller retries a case-creation request.
- [ ] Recover action rows left in `running` after a worker crash by using a bounded claim lease.
- [ ] Record every action attempt, response summary, error classification, and final outcome.
- [ ] Redact secrets and unnecessary Discord payload data from action attempts and audit metadata.
- [ ] Keep failed enforcement from invalidating or hiding the case.
- [ ] Add a failed-action review query and backend response.
- [ ] Add manual retry service and storage behavior.
- [ ] Recheck actor permission, bot permission, target membership, and hierarchy before manual retry.
- [ ] Add failure-dismissal service and storage behavior without deleting attempt history.
- [ ] Add retry, dismiss, and void backend endpoints.
- [ ] Add immutable audit entries for retry requests, retry results, dismissals, and voids.
- [ ] Add timeout-removal as a staff-confirmed reversal operation, not a template action.
- [ ] Add unban as a staff-confirmed reversal operation, not a template action.
- [ ] Require matching Discord permission and hierarchy for reversals.
- [ ] Attach reversal attempts and results to the original case and accepted appeal where applicable.
- [ ] Add mocked Discord tests for timeout, kick, ban, timeout removal, unban, rate limits, retries, and ambiguous failures.
- [ ] Add action idempotency and crash-recovery integration tests.

## Member Notifications

- [ ] Replace generated `send_dm` action rows with one case-level notification workflow.
- [ ] Remove separate level and action DM paths that can notify twice for one case.
- [ ] Let only the selected level decide whether the member is notified.
- [ ] Prevent moderators from changing notification behavior during case creation.
- [ ] Prepare or open the member DM channel before kick or ban when needed.
- [ ] Send the single structured notification after the enforcement outcome is known.
- [ ] Include guild name, official reason, visible context, outcome, case reference, and appeal access.
- [ ] Include the optional guild-specific introduction and footer without supporting arbitrary executable templates.
- [ ] Clearly state whether Discord enforcement succeeded, failed, or requires staff review.
- [ ] Record notification delivery separately from enforcement outcome.
- [ ] Keep DM failure from changing case validity or member dashboard visibility.
- [ ] Surface DM delivery failures to staff without creating a second punitive action failure.
- [ ] Make notification sending idempotent so worker and request retries cannot send duplicates.
- [ ] Ensure action retry does not resend the original case notification unless staff explicitly requests another notice.
- [ ] Add notification rendering tests for case-only, timeout, kick, ban, failed enforcement, appealable, non-appealable, and DM-failure cases.
- [ ] Add integration tests proving one case produces at most one automatic member notification.

## Discord Moderator Experience

- [ ] Remove the free-form reason option from `/case add`.
- [ ] Add structured template-context prompts to `/case add`.
- [ ] Use Discord modals or components for context that cannot fit slash-command options.
- [ ] Keep template selection autocomplete limited to active templates.
- [ ] Show required-context validation errors privately.
- [ ] Keep successful case summaries public in the invoking staff channel.
- [ ] Keep public case summaries limited to case number, target, template, level, and action status.
- [ ] Update the public case summary after asynchronous action and notification outcomes are known.
- [ ] Add `/case view` for authorized staff case detail.
- [ ] Add `/case list` for authorized staff case browsing.
- [ ] Add `/case user` for authorized member history.
- [ ] Add stable case pagination buttons.
- [ ] Add case detail embeds that separate validity, action result, appeal state, context, and evidence links.
- [ ] Add member-history embeds without exposing other guilds or hidden technical details.
- [ ] Add a Discord failed-action review view.
- [ ] Register real retry, dismiss, and void button handlers.
- [ ] Require a void reason through a modal before voiding from Discord.
- [ ] Register the Discord message context command for evidence-backed case creation.
- [ ] Add an appeal entry button or secure dashboard link to eligible case notifications.
- [ ] Add Discord views for staff to inspect appeal status and case-linked review history.
- [ ] Add Discord audit-mirror rendering for important moderation events.
- [ ] Keep Discord audit-mirror messages separate from general logging output.
- [ ] Remove legacy direct moderation commands after migration.
- [ ] Prevent v4 and v5 command-name collisions during coexistence.
- [ ] Remove placeholder component and modal handlers as real workflows are registered.
- [ ] Add interaction deduplication using Discord interaction IDs.
- [ ] Add tests for command definitions, permissions, autocomplete, context modals, message commands, pagination, buttons, deferred edits, and failure recovery.

## Authentication and Backend API

- [ ] Add member-authenticated routes for listing and reading the caller's own cases.
- [ ] Add member-authenticated routes for creating, reading, and updating the caller's appeal.
- [ ] Keep member access independent of current guild membership when a case targets their Discord ID.
- [ ] Add staff routes for case voiding, failed-action review, retry, dismissal, reversals, and staff statistics.
- [ ] Add admin routes for template import, export, restore, context definitions, and guild settings.
- [ ] Add backend routes for audit-mirror configuration.
- [ ] Add backend routes for optional-module settings and status without implementing dashboard UI.
- [ ] Standardize structured API error responses with stable error codes, request IDs, and correlation IDs.
- [ ] Keep status codes and error bodies consistent across validation, authentication, authorization, conflict, and dependency failures.
- [ ] Add complete contract tests for staff, admin, member, former-member, and unauthenticated API access.
- [ ] Document the dashboard-facing request and response contracts maintained by this repository.
- [ ] Add configurable CORS origins for non-local environments.
- [ ] Validate production CORS configuration and fail closed for unknown origins.
- [ ] Add CSRF protection for cookie-authenticated mutating requests.
- [ ] Set explicit secure cookie attributes for production, including `Secure`, `HttpOnly`, and an intentional `SameSite` policy.
- [ ] Add session revocation behavior for logout, compromised sessions, and account changes.
- [ ] Add Discord OAuth token refresh or a clear forced-reauthentication flow.
- [ ] Handle revoked Discord OAuth grants without returning internal errors.
- [ ] Add rate limits for OAuth, member reads, template writes, case creation, retries, and evidence capture.
- [ ] Add HTTP request-body limits appropriate for JSON and evidence metadata.
- [ ] Add HTTP read-header, read, write, and idle timeouts.
- [ ] Add standard security headers where appropriate for the API.
- [ ] Log authentication failures and permission denials with trace IDs without logging tokens or cookies.
- [ ] Redact OAuth tokens, session IDs, cookies, and secrets from logs and error payloads.
- [ ] Add API idempotency for case creation and other externally retried writes.
- [ ] Add pagination limits and defensive query bounds to every list endpoint.
- [ ] Add malformed JSON, oversized request, expired session, revoked token, and cross-guild access tests.

## Appeals

- [ ] Replace placeholder appeal records with the final case-linked appeal model.
- [ ] Add guild-configurable appeal questions and Quack's default appeal form.
- [ ] Validate appeal question ordering, required fields, and supported simple input types.
- [ ] Snapshot the appeal form used when a member submits an appeal.
- [ ] Snapshot case appealability so later template edits do not change existing cases.
- [ ] Enforce at most one appeal record per case with a database uniqueness constraint.
- [ ] Allow only the target Discord identity to create and read a member-facing appeal.
- [ ] Allow banned and departed members to appeal after Discord authentication.
- [ ] Reject appeals for non-appealable, voided, or unrelated cases with clear errors.
- [ ] Add appeal submission, staff response, request-more-information, reopen, accept, reject, and close behavior.
- [ ] Reopen the existing appeal for more information instead of creating another appeal.
- [ ] Require `Moderate Members` or higher for appeal review.
- [ ] Record member and staff appeal events as an immutable timeline.
- [ ] Hide staff identities from the member-facing appeal history while retaining them in audit data.
- [ ] Make accepting an appeal atomically void the case.
- [ ] Keep rejected or closed appeals from changing case validity.
- [ ] Offer timeout removal or unban as a separate staff-confirmed operation after acceptance.
- [ ] Never silently reverse Discord enforcement when an appeal is accepted.
- [ ] Notify the member when staff request information or decide the appeal.
- [ ] Add Discord notification entry links for eligible appeals.
- [ ] Add staff appeal queue and filtering backend behavior.
- [ ] Audit appeal reads, submissions, status changes, decisions, and reversal requests.
- [ ] Add concurrency tests for duplicate submissions and simultaneous appeal decisions.
- [ ] Add end-to-end tests for accepted, rejected, reopened, closed, and failed-reversal appeals.

## Audit Log and Staff Statistics

- [ ] Define the complete set of audit action names and metadata contracts.
- [ ] Record the correct source for dashboard, Discord, system, import, and honeypot activity.
- [ ] Audit every meaningful successful write.
- [ ] Audit every failed or denied sensitive write.
- [ ] Audit permission-sensitive case, history, appeal, template, settings, and module reads.
- [ ] Audit action attempts, retries, dismissals, reversals, and terminal outcomes.
- [ ] Audit template archive, restore, import, and export.
- [ ] Audit member appeal activity and staff decisions.
- [ ] Audit optional-module configuration and automated honeypot cases.
- [ ] Enforce append-only audit behavior in application and storage interfaces.
- [ ] Prevent normal repository methods from updating or deleting audit rows.
- [ ] Redact secrets, tokens, private transport payloads, and unnecessary personal data from audit metadata.
- [ ] Allow every moderator to read the complete guild audit log.
- [ ] Add audit filters for actor, source, action, resource, result, case, member, and date.
- [ ] Add stable audit pagination and ordering.
- [ ] Add audit-mirror delivery to a configured staff-only Discord channel.
- [ ] Keep audit-mirror delivery failures visible without blocking the original moderation operation.
- [ ] Add repair behavior when the audit-mirror channel is removed or becomes inaccessible.
- [ ] Derive staff case, action, appeal, and outcome statistics from existing records.
- [ ] Add time-range, template, action, and result breakdowns without creating leaderboards.
- [ ] Add backend statistics responses for authorized staff use.
- [ ] Keep statistics guild-scoped and prevent cross-guild aggregation of member history.
- [ ] Add tests for audit immutability, redaction, permissions, filtering, mirroring, and statistics calculations.

## Optional Ticket Module

- [ ] Write a module-specific product definition for ticket settings, lifecycle, permissions, transcript retention, and Discord behavior.
- [ ] Add independent per-guild ticket-module enablement and settings.
- [ ] Add ticket entry-channel and private-thread configuration.
- [ ] Create private Discord ticket threads or channels with validated staff/member permissions.
- [ ] Add ticket open, resolve, cancel, and authorized reopen behavior defined by the module specification.
- [ ] Keep tickets separate from cases, actions, appeals, escalation, and member moderation history.
- [ ] Add ticket event timelines without reusing case-note behavior.
- [ ] Capture ticket transcripts when a ticket closes.
- [ ] Add authorized transcript read or export behavior.
- [ ] Add Discord ticket creation, queue, view, reply, and close components.
- [ ] Add backend APIs for ticket settings, status, queue, detail, and transcript access.
- [ ] Audit ticket settings and lifecycle changes.
- [ ] Handle missing or deleted ticket channels and parent channels.
- [ ] Add ticket rate limits and duplicate-open protection.
- [ ] Add module-specific v4 ticket migration with dry-run and idempotency support.
- [ ] Add permission, privacy, lifecycle, transcript, Discord, and migration tests.

## Optional General Logging Module

- [ ] Write a module-specific product definition for logged events, channel routing, privacy, formatting, and retention boundaries.
- [ ] Add independent per-guild general-logging enablement and settings.
- [ ] Configure staff-only Discord destinations for message, member, moderation, and server events.
- [ ] Validate logging-channel permissions and repair deleted channel references.
- [ ] Add a bounded in-memory message cache for edit and delete context.
- [ ] Make cache limits configurable and safe for large guilds.
- [ ] Log configured message edits with before/after content when available.
- [ ] Log configured message deletions and bulk deletions when cached context is available.
- [ ] Log configured attachment and embed metadata without permanently archiving file contents.
- [ ] Log configured member join and leave events.
- [ ] Log configured Discord ban and unban events separately from Quack's audit log.
- [ ] Log configured guild or channel changes selected by the module definition.
- [ ] Keep general logging output in Discord instead of building a searchable permanent event archive.
- [ ] Keep general logging failures and retries separate from case action execution.
- [ ] Add Discord rate-limit handling and bounded retries for log delivery.
- [ ] Redact tokens, webhook secrets, and content excluded by module privacy settings.
- [ ] Add module-specific v4 logging-settings migration with dry-run and idempotency support.
- [ ] Add cache, event, formatting, permission, retry, privacy, and migration tests.

## Optional Honeypot Module

- [ ] Write a module-specific product definition for honeypot setup, trigger rules, exemptions, and safety behavior.
- [ ] Add independent per-guild honeypot enablement and settings.
- [ ] Allow admins to select a honeypot channel and one active case template.
- [ ] Validate that the selected template remains active and compatible with automated use.
- [ ] Handle message events in configured honeypot channels.
- [ ] Ignore Quack, other bots, and exempt staff according to the module definition.
- [ ] Prevent recursive triggers from Quack responses or logging output.
- [ ] Apply the configured template through the normal case creation transaction.
- [ ] Mark honeypot cases with an explicit automated source.
- [ ] Use the normal escalation, action, notification, evidence, permission, and audit records.
- [ ] Define how actor permissions are represented for system-created honeypot cases without granting a fake staff identity.
- [ ] Handle selected-template archive and honeypot-channel deletion safely.
- [ ] Add honeypot trigger counts and outcomes to derived module statistics.
- [ ] Add backend APIs for honeypot settings and status.
- [ ] Audit honeypot configuration, triggers, failures, and created cases.
- [ ] Add module-specific v4 honeypot migration with dry-run and idempotency support.
- [ ] Add setup, exemption, loop-prevention, action, failure, and migration tests.

## V4 Migration and Coexistence

- [ ] Define the final v4 historical-case import format.
- [ ] Import v4 cases as clearly labeled historical records.
- [ ] Preserve useful v4 case identity, guild, target, moderator, reason, type, context link, and creation time.
- [ ] Prevent imported v4 cases from contributing to v5 template escalation.
- [ ] Prevent imported records from creating v5 action executions or member notifications.
- [ ] Keep imported records readable in authorized staff and member history views.
- [ ] Decide how legacy moderator identities are displayed when the user is no longer available.
- [ ] Add dry-run import reports before any data is written.
- [ ] Add import validation with per-record warnings and failures.
- [ ] Make imports idempotent and safely repeatable.
- [ ] Record import batches, source identifiers, counts, failures, and checksums.
- [ ] Audit every import batch and material import failure.
- [ ] Keep v4 and v5 database schemas and Redis keys isolated during coexistence.
- [ ] Add command-scope checks so v4 and v5 do not register conflicting slash commands.
- [ ] Document the transition that removes v4 direct moderation commands after migration.
- [ ] Add module-specific migration hooks without making core migration depend on unfinished modules.
- [ ] Add migration fixtures covering warnings, timeouts, kicks, bans, departed members, missing users, and malformed legacy rows.
- [ ] Add migration rollback and rerun tests.
- [ ] Add a real-data dry-run checklist that does not expose member data in logs.

## Database and Storage Reliability

- [x] Replace production reliance on startup-only `AutoMigrate` with versioned, reviewable database migrations.
- [x] Remove startup `AutoMigrate` instead of retaining a separate local-development schema path.
- [ ] Add forward and rollback migration procedures for every v5 model realignment.
- [x] Test the migration foundation against the current pre-ledger v5 schema and representative stored data.
- [x] Preserve table names, IDs, case numbers, snapshots, action attempts, events, and audit history through the migration foundation.
- [ ] Add database constraints for one default level per template where feasible.
- [ ] Add database constraints for zero or one enforcement action per level where feasible.
- [ ] Add database constraints for one appeal per case.
- [ ] Add uniqueness and index coverage for template identity/version, guild case numbers, member history, audit filters, action claims, evidence, and module settings.
- [ ] Add mapper tests for every new or changed domain/storage record.
- [ ] Add MySQL integration tests for JSON fields, indexes, foreign keys, locks, and transaction rollbacks.
- [ ] Add integration tests proving concurrent case creation keeps unique numbers and correct escalation after the model changes.
- [ ] Add integration tests proving void and appeal acceptance cannot race with escalation counts.
- [ ] Add claim-lease recovery for action executions left running after crashes.
- [ ] Add safe cleanup or archival policies for expired OAuth state and sessions.
- [ ] Define backup and restore procedures for MySQL.
- [ ] Define the required Redis durability and recovery behavior for sessions and command cache.
- [ ] Test backup restoration into a clean environment.
- [ ] Verify that restoring storage cannot duplicate action execution or case numbering.

## Queue, Concurrency, and Recovery

- [ ] Add a bounded lease or heartbeat to database action claims.
- [ ] Recover stale running claims after process termination.
- [ ] Prevent stale workers from completing work after another worker reclaims the action.
- [ ] Add explicit failed-action review state without turning terminal failures into automatic retries.
- [ ] Define when dismissed failures leave the active review queue.
- [ ] Add safe replay tooling for operators without bypassing permission and idempotency checks.
- [ ] Keep persisted actions as the durable source of truth after notification and action model changes.
- [ ] Verify queue polling remains bounded under large pending backlogs.
- [ ] Add fairness tests across guilds and cases so one busy guild cannot starve others.
- [ ] Add duplicate-submit tests for Discord, HTTP, startup recovery, and poller overlap.
- [ ] Add crash tests between Discord success and database completion.
- [ ] Add crash tests between notification delivery and database completion.
- [ ] Add race tests for queue start, submit, poll, stop, retry, reclaim, dismiss, and void interactions.
- [ ] Add operational counters for stale claims, reclaims, safe retries, manual retries, dismissals, and permanent failures.

## Operations, Security, and Deployment

- [ ] Separate liveness from readiness checks.
- [ ] Include database, Redis, Discord, queue, migration, and action-capability readiness.
- [ ] Report guild-scoped degraded status when required Discord permissions or managed channels are unavailable without taking healthy guilds offline.
- [ ] Add metrics for case creation, escalation levels, action attempts, failures, retries, notifications, appeals, audit mirroring, and optional modules.
- [ ] Add alerting guidance for queue backlog, stale running actions, repeated Discord failures, database failures, and migration failures.
- [ ] Add structured logs for authentication, authorization, case, action, appeal, evidence, migration, and module workflows.
- [ ] Keep request and correlation IDs on all new HTTP, Discord, queue, audit, and module paths.
- [ ] Add log redaction tests for tokens, cookies, session IDs, webhook URLs, member content, and action payloads.
- [ ] Validate all required production configuration at startup with actionable errors.
- [ ] Add configuration for CORS, HTTP timeouts, rate limits, managed channels, notification branding, and module toggles.
- [ ] Document production Discord application settings, OAuth redirects, intents, and bot permissions.
- [ ] Document production MySQL and Redis sizing, persistence, backup, and recovery expectations.
- [ ] Add container and deployment resource limits and graceful termination guidance.
- [ ] Verify shutdown behavior while actions, evidence copies, audit mirrors, and optional-module deliveries are active.
- [ ] Add dependency vulnerability scanning such as `govulncheck` to CI or release checks.
- [ ] Add secret scanning and prevent committed `.env` or credential files.
- [ ] Review HTTP and Discord error messages to prevent internal details or personal data leaks.
- [ ] Add an operator runbook for failed migrations, Discord outages, Redis outages, MySQL outages, queue backlog, and stuck actions.
- [ ] Add an operator runbook for manually retrying, dismissing, or voiding failed moderation work.
- [ ] Add a production rollback procedure that preserves cases and prevents duplicate action execution.

## Testing and Release Readiness

- [ ] Update core unit tests to the final no-window, no-severity, no-weight, archive-only, one-action model.
- [ ] Add table-driven tests for every permission and hierarchy rule.
- [ ] Add template validation tests for context fields, archive/restore, import/export, timeout duration, ban deletion, and retry count.
- [ ] Add case tests for context, evidence, target validation, voiding, replacement, member visibility, and action-independent validity.
- [ ] Add action tests for every Discord result, retry classification, idempotency boundary, manual control, and reversal.
- [ ] Add notification tests proving one message at most and accurate outcome rendering.
- [ ] Add appeal tests for ownership, one-per-case, reopen, decisions, voiding, and reversals.
- [ ] Add audit tests for completeness, denied operations, read events, redaction, immutability, and all-moderator access.
- [ ] Add backend API contract tests for every staff, admin, member, and module endpoint.
- [ ] Add full Discord interaction tests for slash commands, message commands, autocomplete, components, modals, deferred edits, and public/private responses.
- [ ] Add Redis integration tests for OAuth state, sessions, command cache, expiry, and unavailable Redis behavior.
- [ ] Add MySQL integration coverage for schema migrations, locks, JSON, constraints, and concurrency.
- [ ] Add optional-module unit, integration, Discord, privacy, and migration tests.
- [ ] Add end-to-end tests for template creation, case creation, escalation, evidence, enforcement, notification, audit, member read, appeal, and voiding.
- [ ] Add end-to-end tests for safe retry, unsafe manual retry, dismissal, reversal, and action crash recovery.
- [ ] Add Docker Compose smoke tests for startup, migration, `/status`, readiness, OAuth prerequisites, command sync, `/ops/status`, and shutdown.
- [ ] Add a real test-guild checklist covering install, starter policy, permissions, hierarchy, evidence channel, case creation, each action, member DM, member case access, appeal, audit mirror, and recovery.
- [ ] Run targeted `go test -race` coverage for the queue, action claims, case creation, appeal decisions, and evidence capture.
- [ ] Add `go vet ./...` to CI.
- [ ] Add race, integration, and migration jobs to CI with MySQL and Redis services.
- [ ] Keep `go test ./...`, `go vet ./...`, and `go build ./cmd/quack` as required release gates.
- [ ] Add coverage reporting that fails when critical core packages lose meaningful behavioral coverage.
- [ ] Add fuzz tests for Discord message links, custom IDs, imported template files, structured context, and legacy import rows.
- [ ] Verify Compose and production container images use the pinned supported Go version.
- [ ] Update architecture, module, API, configuration, testing, migration, operations, and release documentation as implementation changes land.
- [ ] Complete a final scope-drift audit against every rule in `v5.md`.
- [ ] Complete a security review of authentication, authorization, evidence exposure, audit data, and member privacy.
- [ ] Complete a controlled v4/v5 coexistence rehearsal and rollback test.
- [ ] Complete a clean-install rehearsal in a new guild.
- [ ] Complete an upgrade rehearsal from the current v5 schema with existing cases and pending actions.
- [ ] Produce the final backend release checklist and record its results.
