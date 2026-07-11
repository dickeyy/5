# Quack v5

Quack v5 is a customizable moderation system for Discord. Administrators
define templates and escalation levels; moderators apply a template; Quack
selects the matching level, records the case, and carries out its configured
outcome.

The dashboard and Discord bot are delivery adapters over the same application
core.

The authoritative product definition is [`v5.md`](v5.md). The current backend
does not yet implement every rule in that document; the high-level differences
are tracked in [`docs/v5-scope-drift.md`](docs/v5-scope-drift.md).

## Layout

- `cmd/quack`: process entrypoint.
- `internal/runtime`: dependency assembly and lifecycle.
- `internal/quack`: domain models, ports, and application services.
- `internal/store`: MySQL/GORM and Redis adapters.
- `internal/httpapi`: dashboard-facing HTTP adapter.
- `internal/discordbot`: Discord commands, interactions, and UI.
- `internal/workqueue`: in-process workers backed by persisted action rows.

## Development

Copy `.env.example` to `.env`, start MySQL and Redis with
`docker compose up -d`, then run:

```sh
go run ./cmd/quack
```

Run the validation suite with:

```sh
go test ./...
go vet ./...
go build ./cmd/quack
```
