# Optional honeypots

This document is the module-specific product definition required by Quack v5.
It refines the `v5.md` honeypot boundary without adding honeypot state or special
cases to the moderation core.

## Configuration and permissions

Honeypots are enabled independently for each guild through the optional-module
registry. A configuration selects exactly one Discord trap channel, exactly one
active automation-compatible case template, and zero or more exempt Discord
roles. Manage Guild is required to read status, change settings, or repair a
disabled configuration. Enabling validates that Quack can observe the selected
channel and that the selected template is live and compatible with unattended
application. Configuration never stores a moderator identity.

One message-create event in the selected channel is a trigger regardless of
message content, embeds, or attachments. Quack itself, every bot, webhooks,
members with Moderate Members or stronger authority, and configured exempt
roles are ignored. Events from other channels are ignored. These rules prevent
Quack responses, logging output, integrations, and staff setup activity from
recursively triggering automation. The integration adapter must derive staff
authority and role membership from the current Discord member and must not
trust message-supplied claims.

The module needs only the guild and guild-message gateway intents while at
least one guild enables it. It does not inspect message content and therefore
does not require the privileged Message Content intent. Disabled installations
add no honeypot-specific intent requirement.

## Normal moderation path and safety

Before any moderation side effect, `(guild_id,message_id)` is claimed in the
honeypot trigger ledger. Discord replays and concurrent deliveries therefore
cannot create multiple cases. A qualifying trigger calls the injected QP-A
application interface with:

- the configured template and message author as target;
- source `honeypot`;
- actor type `system` and no fabricated staff user ID;
- the trap channel, message ID, jump URL, and a deterministic idempotency key.

The QP-A adapter must call the ordinary case application service, not case
storage. Template escalation, the selected action, notification, evidence
capture, immutable events, action scheduling, and core audit records are thus
created by the same transaction and workers used by dashboard and Discord
moderation. The honeypot package never executes Discord enforcement itself.

A normal-path failure marks the trigger failed and emits a system-attributed
module audit failure. It does not mutate or retry the case action queue and does
not affect another message, guild, or module. Automatic replay remains blocked
because a retry after an ambiguous failure could duplicate enforcement; staff
inspect the visible failure count and apply the template manually if needed.

Template availability is revalidated immediately before application. If the
selected template is archived, missing, or no longer compatible, the trigger
fails and the module disables while retaining both references and a visible
reason. Deleting the trap channel also disables only that guild's honeypot.
Repair revalidates the retained channel and template before re-enabling. A
transient QP-A/application failure increments failures but does not disable an
otherwise healthy configuration.

## Status, audit, privacy, and retention

The backend registrar exposes manager-only settings, status, update, and repair
routes under the caller's authenticated guild module group. Status includes
enabled/configured state, retained drift reason, and guild-scoped counts for
created, failed, and exempt trigger outcomes. Counts are derived from the
module's trigger ledger rather than from a second copy of cases.

The trigger ledger stores Discord identifiers, selected template, resulting
case ID, outcome, and a bounded failure code. It never stores message content,
attachments, action payloads, or member history. Evidence belongs to the normal
case snapshot. Settings reads denied by current authority, configuration
changes, trigger detection, failures, drift disables, migration, and successful
case creation are sent to the caller-provided immutable core audit sink.

Logical migration 0300 creates only `honeypot_triggers`. The package exports the
migration, descriptor, HTTP registrar, Discord adapter, intent requirements,
and independently drainable bounded runtime; QI-2 owns central registration.
Closing the honeypot runtime drains accepted messages and cannot close ticket,
logging, case-action, or notification workers.

## V4 migration and isolation

The v4 importer accepts only an explicit settings shape. Dry-run validates the
guild, channel, template, and payload and performs no writes. Real import uses
the shared `(guild,module,source)` ledger and atomically upserts only the
honeypot configuration, so reruns do not duplicate state. It never imports
legacy triggers as cases, actions, evidence, audit history, or escalation input.

Tickets, general logging, and honeypots have independent registry envelopes,
data tables, adapters, failure paths, migrations, permissions, and worker
lifecycles. Honeypot channel deletion, template drift, a full worker queue, or a
failed action path cannot change ticket/logging settings or core moderation
state. Cross-guild fallback is forbidden by the registry and every trigger and
statistics query is guild-scoped.

## QI-2 integration contract

QI-2 must:

1. add `Descriptor()` to the shared registry and `Migration()` to the ordered
   migration ledger using logical version 0300;
2. adapt live template/channel validation and immutable module auditing;
3. implement `CaseApplier` by invoking QP-A's system case-application entrypoint
   with the request unchanged;
4. project Discord messages with authoritative bot, webhook, staff, and role
   facts, then submit them to the bounded runtime;
5. forward channel deletion and template archive/incompatibility events to the
   adapter;
6. mount `RegisterRoutes` beneath authenticated guild module routes; and
7. request the exported intents only when an enabled configuration exists and
   call `Close` during graceful shutdown.

Live-guild validation remains a release gate because it requires Discord
credentials and permission-changing external operations.
