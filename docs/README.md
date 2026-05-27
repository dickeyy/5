# Docs

Internal maintainer docs for the Quack v5 backend.

This directory covers the code that exists in this checkout today. Product-roadmap
documents such as `v5.md` and the dashboard handoff in `website-agent-plan.md`
remain the higher-level planning references.

## Index

- `architecture.md`: service layout, startup flow, request flow, Discord interaction flow, and action execution.
- `configuration.md`: environment variables and runtime dependencies.
- `development.md`: local workflow, Docker usage, commands, and where to make common changes.
- `modules/README.md`: focused notes for core runtime modules and pipelines.
- `testing.md`: current test harness and scope limits.

## Current Surface

The live backend currently has four main runtime surfaces:

- Discord bot startup, slash-command registration, and interaction dispatch in `main.go`, `discord/commands/`, and `discord/interactions/`.
- HTTP API routes for status, auth, guild context, templates, and case creation in `api/server.go` and `api/routes/`.
- Case-action queue processing in `services/event-queue.go` and `app/actions_queue.go`.
- Local container packaging for MySQL, Redis, and the app profile in `compose.yaml` and `Dockerfile`.

Relevant files:

- `main.go`
- `api/server.go`
- `api/routes/router.go`
- `discord/commands/case.go`
- `discord/interactions/dispatcher.go`
- `discord/ui/message.go`
- `app/actions_queue.go`
- `compose.yaml`
