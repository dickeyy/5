# Quack v5

Quack is a configurable moderation policy engine for Discord. Administrators
define templates and escalation levels; moderators apply a template; the
backend selects the matching level, records the case, and executes its actions.

The dashboard and Discord bot are delivery adapters over the same application
core.

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
