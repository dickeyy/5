# Optional tickets and general logging

This document is the module-specific product definition required by Quack v5.
It refines the boundaries in `v5.md`; it does not extend the moderation core.
Both modules are enabled and configured independently for each guild through
`internal/modules.Registry`. Their configuration rows, routes, Discord adapters,
imports, and runtime failures cannot create or mutate cases, actions, appeals,
escalation history, or member moderation history.

## Shared module boundary

`module_configurations` stores one canonical enablement envelope per guild and
module. The opaque JSON payload is validated by the owning module. A registry
rejects duplicate descriptors, unknown modules, and cross-guild fallbacks.
Tickets, general logging, and honeypots have distinct identifiers, so disabling
or repairing one never changes another. The pre-module booleans in guild setup
are compatibility inputs for the integration package; integrated runtime code
must map them once into these canonical envelopes rather than maintain two
sources of truth.

Module configuration and sensitive module reads use the same live Discord
authority boundary as the dashboard. Manage Guild configures either module.
Moderate Members operates and reads the staff ticket queue. A ticket owner may
read and reply only to their own ticket. Configuration changes, denials, ticket
lifecycle changes, permission repairs, and imports emit immutable entries to a
caller-provided core audit sink. General Discord event deliveries never do.

The package exports route and Discord registrars plus migrations instead of
editing central runtime, router, command, or migration registries. Integration
owns those central files. Migration 0100 creates the registry and module import
ledger; migration 0110 adds ticket transcript storage and upgrades the existing
ticket/event tables. Module migrations are exposed as descriptors for ordered
integration into the production ledger.

## Tickets

### Settings and permissions

A guild selects one entry channel, one or more staff roles, and private-thread
mode (the default; a client may implement equivalent private channels). The bot
validates that only the owner, configured staff, and bot can view each ticket.
Tickets default to one open ticket per member, three opens per rolling 24 hours,
a seven-day staff reopen window, and 90-day transcript retention. Administrators
may configure daily limits from 1–20, reopen windows from 1–720 hours, and
retention from 1–365 days.

The entry channel is only an initiation surface. Ticket content is never posted
there. Losing it disables new ticket creation and clears the stale reference.
Losing a ticket channel records a `channel_missing` timeline event and preserves
backend state and any captured transcript for authorized recovery. Permission
repair reapplies the exact private ACL and records a repair event.

### Lifecycle and Discord behavior

Opening creates one private channel/thread and an `open` ticket. The adapter
archives a newly provisioned channel if ACL setup or backend creation fails.
Owners and staff can reply; the queue, arbitrary detail, close, reopen, and
permission repair operations require current staff authority. Owners may cancel
their own open tickets. Staff resolve an open ticket. Staff may reopen resolved
or cancelled tickets only inside the configured window. Every transition and
reply appends an immutable, ordered ticket event rather than a case note.

The Discord integration exposes entry/open, staff queue/view, reply, close,
permission repair, deleted-ticket-channel, and deleted-entry-channel operations.
Closing captures a transcript before channel archival. Backend status reports
whether the entry is configured and the current open count. API registrars expose
settings, status, queue, detail, transcript, resolve, cancel, and reopen under a
caller-provided authenticated guild group.

### Privacy, retention, limits, and recovery

Ticket details, events, and transcripts are guild-scoped. Only the owner or
current staff can read content. Audit metadata does not include reply or
transcript bodies. Transcripts are stored separately from ticket events, expire
at the configured deadline, and are deleted by the exported retention sweep.
They are not searchable moderation records and do not contribute to staff or
member moderation statistics.

Duplicate and rolling-day checks run in the ticket persistence transaction.
Discord failures do not change moderation actions. A failed close leaves the
private channel available for retry; a deleted channel leaves a durable recovery
event. Repeated component deliveries are constrained by ticket state, and
invalid transitions fail without adding events.

### V4 migration

The ticket importer accepts an explicit ticket-only record shape. Dry-run
validates every guild, identity, and status without writes. Real imports use the
shared `(guild,module,source)` ledger, so reruns return the original target and
do not duplicate tickets or events. Imported rows are labeled in module metadata
and audited once per run. The importer never maps tickets into historical cases,
appeals, notes, actions, or escalation.

## General logging

### Event and routing definition

Guilds independently route any of these events to staff-only Discord channels:
message edit, message delete, bulk delete, member join, member leave, Discord
ban, Discord unban, selected guild changes, and selected channel changes. Ban
and unban events describe Discord gateway activity; they are not Quack case
actions or audit entries. Each event has one configured destination, while
multiple categories may share a destination.

Destinations are validated as staff-only before configuration is accepted.
Deleting a destination removes every route to it and disables the module if no
routes remain. Settings and transient delivery health are available through the
module route registrar. Health contains only counters and the latest delivery
error, not an event archive.

### Privacy, format, cache, and retention

Delivery is structured JSON containing the event type and Discord identifiers.
Message content is excluded by default and included only when configured.
Attachment logging is limited to filename, content type, and size; embed logging
is limited to embed type. Files and embeds are never downloaded or archived.
Token-like values, authorization secrets, and Discord webhook URLs are redacted
from content and metadata before transport.

A concurrency-safe in-memory cache supplies before-content for edits and deletes
and cached context for bulk deletes. It is isolated per guild, defaults to 1,000
messages per guild, is configurable from 1–10,000, and evicts oldest entries
immediately when reduced. Cache state disappears on restart by design. General
logging writes no permanent event table and provides no search API.

### Delivery, retry, and recovery

Configured events are delivered to Discord with one to five bounded attempts
(three by default). Discord retry-after delays take precedence over the bounded
local backoff. Cancellation stops retry promptly. Failures update module health
but never block, retry, dismiss, or otherwise affect a case action. Runtime
integration may queue gateway events, but its queue must remain bounded and may
drop general logs under pressure rather than delay moderation.

The v4 importer accepts settings only—never historical log messages. Dry-run is
side-effect free. Real imports atomically upsert the guild's logging envelope
and write an idempotency ledger entry, and configuration import is audited. A
rerun performs no additional configuration or audit-sensitive event import.

## Integration contract

The integration package must:

1. apply the exported migrations in the production ordered ledger;
2. construct one shared registry with the ticket and general-logging descriptors;
3. adapt the core immutable audit writer to `modules.Auditor`;
4. mount each route registrar under the authenticated guild group and translate
   the live guild context into its module actor;
5. register the ticket Discord adapter and general-logging gateway handlers;
6. run the transcript retention sweep and bounded event worker lifecycle; and
7. preserve the rule that general-log deliveries never enter the audit table.

`internal/moduleintegration` implements this contract in production. It reuses
the shared HTTP platform safety primitives, installs ticket components in the
single command dispatcher, keeps gateway delivery off the moderation queue,
drains accepted work during shutdown, and runs transcript cleanup without
deleting ticket timelines.

Real-guild validation remains an integration/release gate because it requires
Discord credentials and permission-changing external operations.
