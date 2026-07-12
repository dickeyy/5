# QI-2 P2 integration manifest

QI-2 composes the P2 capability packages on accepted QI-1 head `11650a5`.
The current checkpoint contains accepted QP-F head `fc6d82c`; QP-D and QP-E
remain intentionally unmerged until their orchestrator acceptance handoffs.

## Installed QP-F contracts

- `honeypot.Descriptor()` joins the one process registry and logical migration
  0300 is frozen as checksum-bound physical migration 0008 immediately after
  existing 0007. A later QP-D merge must reconcile logical 0200 into the next
  contiguous physical ledger position without editing an applied definition.
- Manager-only settings, status, update, and repair routes are mounted beneath
  the authenticated live-guild module group and reuse the shared rate-limit,
  idempotency, and error-envelope boundaries.
- Channel and template validators read current Discord/core state. Gateway
  message projections reload the current member, roles, channel permissions,
  bot identity, and guild state instead of trusting message-supplied claims.
- `honeypotCaseApplier` preserves QP-F's request into the narrow
  `CaseService.CreateSystemHoneypot` entrypoint. The entrypoint reuses the
  ordinary template compatibility, escalation, evidence, snapshot,
  transaction, action, notification, scheduling, idempotency, and bot/target
  safety paths while persisting an empty moderator and explicit system
  event/audit attribution. QP-A had no callable system boundary; QI-2 added
  this narrow integration-owned contract and rejects staff or non-honeypot use.
- Template updates/archives and Discord channel/guild deletions forward drift
  to the honeypot adapter. Only the matching guild configuration disables and
  its selected references remain available for repair.
- Startup intents are derived from durable enablement. Honeypots add Guilds and
  Guild Messages without Message Content; general logging adds its member,
  moderation, message, and privileged content requirements only when enabled.
- The isolated bounded honeypot runtime starts during composition, receives
  projected message events, drains accepted work during reverse-order graceful
  shutdown, and cannot close ticket, logging, action, or notification workers.

## Pending accepted-head reconciliation

When QP-D is accepted, QI-2 must instantiate its appeal service, register its
member and staff route registrars, append its logical 0200 migration through
the package's frozen central migration builder, register
`discordbot.RegisterAppealComponents` for `appeal:reverse:v1`, and wire its
notification dispatcher/adapter and appeal-entry view. The exported component
registrar owns confirmed reversal and must retain live permission, bot, target,
and hierarchy checks; QI-2 must not implement a parallel handler.

When QP-E is accepted, QI-2 must register its audit/statistics staff routes and
case components and start/close its bounded audit mirror worker. QP-E declares
no schema migration. Dispatcher ID deduplication must remain intact when both
P2 component registrars join the process. The module audit adapter must also
use QP-E's canonical source resolver: honeypot trigger/case activity maps to
`honeypot`, v4 module imports map to `import`, and other module operations use
the request context's API/Discord/system source instead of today's blanket
system attribution.

After both merges, QI-2 will resolve central registrar/runtime conflicts once,
run the combined focused, migration/MySQL, race, full test, vet, and binary
build gates, then create one P2 integration PR and request one Codex review.

Real Discord permission-changing checks remain a release validation gate that
requires explicitly authorized guild credentials.
