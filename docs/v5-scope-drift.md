# V5 Scope Drift

This document is a high-level comparison between the current backend and the product definition in [`v5.md`](../v5.md).

It is an audit, not a roadmap or implementation plan. A listed mismatch does not mean the behavior has already changed. Technical documentation should continue describing the code that exists until that code is updated.

## Already Aligned

The current backend already supports important parts of the intended v5 direction:

- Templates own levels and their actions.
- Moderators apply templates instead of choosing an escalation level.
- Case creation counts history from the selected template and snapshots the chosen result.
- Cases, action attempts, case events, and audit entries preserve moderation history.
- Discord and the dashboard call the same application behavior.
- Guild data and escalation history are isolated by guild.
- Action rows are persisted so work can recover after queue saturation or process restart.
- Discord interactions support deferred work, public case results, template autocomplete, and reusable UI components.
- Template and case reads are available for staff dashboard workflows.
- Production schema changes use an ordered, checksum-tracked migration ledger
  that adopts current v5 data additively without startup `AutoMigrate`.
- Live template contracts now use all-time distinct thresholds, archive-only
  availability, level-owned notification, and zero or one typed enforcement
  action. Frozen legacy columns remain storage-only compatibility data.
- Live case contracts now use only valid/voided validity and the dashboard,
  Discord, honeypot, and v4-import sources. Action and appeal state remain
  separate, and the official reason always comes from the immutable template
  snapshot source.
- Severity, weight, reason overrides, and free-form note lifecycle are absent
  from live case models and contracts. Migration 0003 preserves legacy columns
  and events, inventories retired event rows, and exposes none of them through
  live v5 case/event reads.
- GuildCreate bootstrap now atomically persists guild-owned channel and
  notification settings, independent optional-module enablement, the exact
  editable starter policy, and its one-time dashboard review notice. Guild
  update, leave, rejoin, and channel-deletion events preserve history while
  refreshing identity and clearing stale configured references.

## Behavior That Must Change

The current implementation differs from the v5 definition in several core rules:

- The current level-owned notification is still represented as a pre-enforcement `send_dm` action. V5 sends one structured notification after the action outcome is known.
- Current case permissions do not fully enforce the actor's matching timeout, kick, or ban permission before case creation.
- The current dashboard guild flow is staff-focused and does not yet provide case-owner access for members who left or were banned.
- Template import, export, and restore are not yet defined in the running product.

## Rejected Concepts to Remove

These concepts appear in current models, documentation, or backlog history but do not belong in the v5 product model:

- Cross-template or cross-guild escalation.
- Direct `/warn`, `/timeout`, `/kick`, and `/ban` workflows after migration.
- Generic bot utilities as requirements for the moderation core.

Historical database compatibility may require a later migration rather than immediate column removal. Keeping old data temporarily does not make the rejected concept part of the product.

## Missing Core Behavior

The intended v5 core still requires product behavior that is not complete today:

- Real Discord timeout, kick, and ban execution.
- Strict target, permission, and role-hierarchy checks before creating a punitive case.
- Safe automatic retry classification and staff-controlled retry, dismiss, and void actions.
- One structured member notification sent after the action outcome is known.
- Structured template context fields shared by Discord and the dashboard.
- Discord message context actions and pasted-link evidence capture.
- Permanent message snapshots and attachment preservation through a managed evidence channel.
- Member-facing case access with the agreed transparency and staff-identity rules.
- Case voiding and replacement behavior.
- A complete case-linked appeal flow.
- Searchable audit views and optional Discord audit mirroring for all moderators.
- Derived staff statistics.
- V4 historical-case import that does not affect escalation.
- Removal of legacy direct moderation commands after migration.

## Optional Modules Must Stay Separate

Some v4 features remain part of Quack v5 as optional guild modules, not as extensions of the case model:

- Tickets remain private Discord support threads with their own lifecycle and transcripts.
- General logging sends selected Discord events to staff log channels and remains separate from the audit log.
- Honeypots may invoke a configured template automatically, but the resulting moderation still uses the normal case, action, notification, and audit flow.

Purge, lockdown, ping, server information, and similar utilities may be considered later. They should not add fields, permissions, or special cases to the core moderation system.

## Using This Audit

Future slices should begin with the relevant rule in `v5.md`. When a slice touches an item above, it should either bring the implementation closer to that rule or update the product definition first when the intended behavior has genuinely changed.

Remove items from this audit only after the code and its technical documentation agree with `v5.md`.
