# Guild Setup and Settings

Discord gateway lifecycle events, not an arbitrary dashboard request, own the
install boundary. `internal/discordbot/guild_lifecycle.go` registers before the
gateway opens and delegates to `GuildService`; `internal/store/guild_settings.go`
performs the atomic install transaction.

## Install and rejoin

A usable `GuildCreate` payload atomically:

1. creates or reactivates the guild and refreshes its name, icon, and owner;
2. creates one settings row when absent;
3. creates and binds exactly one active, editable, appealable
   `General rule violation` template;
4. seeds default notify-only cases 1-2, a notified 24-hour timeout from case 3,
   and a notified ban with 24 hours of message deletion from case 5;
5. leaves the starter-review notice pending; and
6. clears configured core channel references absent from the complete guild
   channel inventory.

The starter template and notice identity are persisted, so repeated events and
leave/rejoin cannot duplicate them. A true guild leave only marks the guild
inactive. Discord's temporary `Unavailable` signal is ignored. No guild-owned
history is hard-deleted.

## Settings contract

`GET /guilds/:discordGuildID/settings` returns core channel references,
notification branding, independent ticket/logging/honeypot enablement, the
starter template ID, and whether dashboard review is still required.
`PATCH /guilds/:discordGuildID/settings` is partial. Empty channel strings clear
references. `POST /guilds/:discordGuildID/settings/starter-policy-notice/acknowledge`
completes the
one-time notice without archiving or disabling the starter policy.

All settings reads and writes require current owner, `Administrator`, or
`Manage Guild` authority. The current request path resolves Discord guild
permissions before service authorization; V5-003 owns the broader live
permission and target-hierarchy hardening. Successful writes are audited
atomically, and validation failures or denied writes produce immutable failure
or denied audit entries.

## Channel boundary

The settings table persists audit-mirror and managed-evidence channel IDs.
`ChannelDelete` clears only matching references, and a complete rejoin inventory
clears stale references. This slice does not create channels, set Discord
overwrites, check permission drift, or upload evidence. Those operations remain
the managed-evidence and audit-mirror slices.
