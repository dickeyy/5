# Docs

Internal maintainer docs for the Quack v5 backend.

This directory covers the code that exists in this checkout today. The
authoritative product definition lives in [`v5.md`](../v5.md). When current code
differs from that definition, [`v5-scope-drift.md`](v5-scope-drift.md) records
the high-level mismatch without making the technical docs inaccurate.

## Index

- `architecture.md`: service layout, startup flow, request flow, Discord interaction flow, and action execution.
- `configuration.md`: environment variables and runtime dependencies.
- `development.md`: local workflow, Docker usage, commands, and where to make common changes.
- `http-api-platform.md`: OAuth/session lifecycle, browser security, stable errors, rate limits, and HTTP idempotency contracts.
- `migrations.md`: production migration ledger, forward, rerun, failure recovery, and rollback procedures.
- `release-readiness.md`: current ops status, tracing, coexistence, and release checklist.
- `v5-scope-drift.md`: high-level differences between the current backend and the intended v5 product.
- `modules/README.md`: focused notes for core runtime modules and pipelines.
- `testing.md`: current test harness and scope limits.

## Current Surface

The live backend currently has four main runtime surfaces:

- Discord bot startup, slash-command registration, and interaction dispatch in `cmd/quack/main.go`, `internal/discordbot/commands/`, and `internal/discordbot/interactions/`.
- HTTP API routes for status, ops status, auth, guild context/settings, templates, case reads and creation, and guild audit reads in `internal/httpapi/server.go` and `internal/httpapi/routes/`.
- Case-action queue processing in `internal/workqueue/queue.go` and `internal/workqueue/queue.go`.
- Local container packaging for MySQL, Redis, and the app profile in `compose.yaml` and `Dockerfile`.

Relevant files:

- `cmd/quack/main.go`
- `internal/httpapi/server.go`
- `internal/httpapi/routes/router.go`
- `internal/discordbot/commands/case.go`
- `internal/discordbot/interactions/dispatcher.go`
- `internal/discordbot/ui/message.go`
- `internal/workqueue/queue.go`
- `compose.yaml`
