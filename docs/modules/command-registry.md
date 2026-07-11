# Command Registry

The command registry is the Discord slash-command assembly point. It owns three
jobs:

- collecting local command specs
- dispatching incoming Discord interactions to handlers
- syncing local definitions to Discord application commands

The core registry type lives in `internal/discordbot/commands/registry.go`.

## Registration Model

Each command contributes a `CommandSpec`:

- `Definition`: `*discordgo.ApplicationCommand`
- `Handler`: function invoked for command and autocomplete interactions

Commands register themselves during package init through `registerCommand(...)`.
Today the default registry only includes `/case`, from `internal/discordbot/commands/case.go`.

`Registry.Register` rejects:

- nil registries
- nil definitions
- empty command names
- nil handlers
- duplicate command names

`Registry.Specs` returns commands sorted by name so downstream sync behavior is
stable.

## Runtime Dispatch

`Register(session, services)` installs one Discord interaction handler on the
session.

Dispatch rules:

- only application commands and autocomplete interactions are considered
- the incoming command name is looked up in the registry
- the command handler receives `context.Background()`, `*quack.Services`, the
  Discord session, and the interaction payload
- if the handler returns a response, `InteractionRespond` sends it back to
  Discord

The registry itself stays thin. Command-specific auth, option parsing, and app
service calls belong in the individual command module, such as
`internal/discordbot/commands/case.go`.

## Sync Model

After installing the interaction handler, `Register` builds a `CommandSyncer`
and calls `Sync(...)`.

`CommandSyncer`:

1. Lists remote commands for the configured app and optional guild scope.
2. Hashes each local command definition with `CommandFingerprint`.
3. Reads an optional cached command ID and hash from Redis.
4. Creates missing commands.
5. Refreshes the cache when remote and local hashes already match.
6. Updates remote commands when canonical hashes differ.
7. Optionally prunes remote-only commands when `PruneEnabled` is true.

The sync logic is in `internal/discordbot/commands/sync.go`.

## Hashing And Cache Semantics

`CommandFingerprint` in `internal/discordbot/commands/hash.go` canonicalizes the Discord
command definition before hashing it with SHA-256.

Normalization details that matter:

- zero-value command type is normalized to chat input
- integration types are sorted
- the default pair of integration types collapses to nil
- false `NSFW` is normalized to nil

The goal is to avoid churn from semantically equivalent Discord definitions.

Redis cache behavior in `internal/discordbot/commands/cache.go`:

- key pattern: `discord:commands:<scope>:hashes`
- field: command name
- value: JSON containing `discord_command_id` and `hash`
- if Redis is unavailable, the registry falls back to a no-op cache

The cache is an optimization, not the source of truth. Remote comparison still
protects correctness.

## Scope And Configuration

`Register` resolves the application ID from `lib.Config.Discord.AppID` or the
connected session user ID. It also passes through:

- `CommandGuildID` for guild-scoped sync
- `CommandPrune` to allow deleting remote-only commands

If the app ID is missing, startup fails when command registration runs.

## Maintainability Notes

- Package `init()` registration keeps command wiring decentralized, but it also
  means import paths must continue pulling command packages into the build.
- `context.Background()` is used for interaction handlers and sync; there is no
  per-request cancellation path from Discord yet.
- Only `/case` is registered right now, so registry complexity is mostly about
  keeping sync deterministic for future commands.

Relevant files:

- `internal/discordbot/commands/registry.go`
- `internal/discordbot/commands/case.go`
- `internal/discordbot/commands/sync.go`
- `internal/discordbot/commands/cache.go`
- `internal/discordbot/commands/hash.go`
- `internal/discordbot/commands/registry_test.go`
- `internal/discordbot/commands/sync_test.go`
- `internal/discordbot/commands/hash_test.go`
