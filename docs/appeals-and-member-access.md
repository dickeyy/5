# Appeals and Member Access

QP-D implements the v5 case-owner and appeal contracts without using current
Discord guild membership as an authorization source. A signed-in member is
authorized by equality between the session Discord user ID and the immutable
case target ID. This keeps access available after a leave or ban and prevents
guild or other-member enumeration.

## Member projections

Member case lists return only the case reference, official reason, validity,
selected outcome, appeal state, and timestamps. Case details add submitted
context, authorized evidence, public history, and a redacted enforcement result.
They never return moderator IDs, raw Discord failures, worker or retry data,
internal event metadata, or staff-only evidence-channel identifiers. Valid and
voided cases remain visible.

## Appeal form and timeline

Guild managers may configure one to ten ordered `short_text`, `long_text`, or
`boolean` questions. Quack supplies a two-question default. Question IDs,
prompts, types, ordering, and required flags are validated, and the effective
form plus validated answers are snapshotted on the single appeal row for a case.
Later settings or template edits cannot change an existing appeal or its
snapshotted appealability.

The timeline is append-only. Submission creates `pending`; staff may request
information, the member may append information to return it to `pending`, and
staff may accept, reject, or close. Rejected or closed appeals may be reopened
for information on the same record. Acceptance records the decision and voids
the case in one transaction. It does not queue a Discord action.

Accepted staff views contain explicit timeout-removal or unban offers only when
the original action succeeded. Execution remains a separate confirmed call to
QP-A's reversal service, which performs the live QP-V5-003 permission, target,
bot-permission, and hierarchy preflight and links the result to the appeal.

## HTTP and Discord integration

`RegisterAppealAndMemberRoutes` mounts member case and appeal routes beneath an
already authenticated `/members/me` group. `RegisterAppealStaffRoutes` mounts
settings, queue, review, decision, and reversal routes beneath an authenticated
guild group. Both require QP-B rate-limit and idempotency primitives; Redis
unavailability fails closed, and replay returns the original response.

QI-2 must replace the older member registrar with
`RegisterAppealAndMemberRoutes`, add `RegisterAppealStaffRoutes`, instantiate
`quack.NewAppealService(services.Store)`, and append
`migration0200Appeals(nextVersion)` to the contiguous central registry. It must
also schedule `AppealNotificationDispatcher`, provide a staff-only destination
resolver, and call `discordbot.RegisterAppealComponents` on the central
component registry. The secure `views.AppealEntryMessage` link belongs in
eligible case notifications. These are integration-owned changes; QP-D does
not edit the central router, runtime, command registry, or migration registry.

Appeal acceptance cancels pending or retrying enforcement and undelivered case
notifications in the same transaction that voids the case. Appeal notification
dispatchers atomically lease outbox rows before sending, reclaim expired leases,
and reject stale completion tokens so concurrent workers cannot double-send.

## Validation

Focused tests cover default and configured forms, immutable snapshots,
ownership and unrelated-user denial, departed-member access, valid and voided
case projections, duplicate submission, every state transition, staff-identity
redaction, atomic acceptance, direct-void/decision races, explicit reversals,
queued-work cancellation, concurrent leased member/staff outbox delivery,
expired-lease recovery, Discord views, HTTP replay, fail-closed Redis,
logical 0200 preservation, and real MySQL schema/transaction behavior.
