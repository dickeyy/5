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
- `dashboard-api-policy-v5.md`: final dashboard/internal endpoint policy matrix.
- `migrations.md`: production migration ledger, forward, rerun, failure recovery, and rollback procedures.
- `operations-security-v5.md`: final health, metrics, configuration, outage, recovery, and shutdown runbook.
- `storage-recovery-v5.md`: MySQL backup/restore manifest and Redis recovery rehearsal.
- `v4-historical-import.md`: final v4 import format, dry-run, repeat, rollback, coexistence, and cutover.
- `v5-rehearsal.md`: local, external-storage, coexistence, restore, and real-guild evidence protocol.
- `v5-readiness.md`: requirement matrix, validation evidence, exceptions, and current READY/NOT READY verdict.
- `release-readiness.md`: compatibility overview linking the final operations and readiness evidence.
- `release-infrastructure-proposal-v5.md`: exact unauthorized CI/container/Compose changes for an infrastructure owner.
- `v5-scope-drift.md`: high-level differences between the current backend and the intended v5 product.
- `modules/README.md`: focused notes for core runtime modules and pipelines.
- `testing.md`: current test harness and scope limits.

## Current Surface

The live backend currently has four main runtime surfaces:

- Discord bot startup, slash-command registration, and interaction dispatch in `cmd/quack/main.go`, `internal/discordbot/commands/`, and `internal/discordbot/interactions/`.
- HTTP API routes for liveness/readiness/metrics, ops status, auth, guild settings,
  templates, cases/recovery, audit/statistics, appeals/member access, and optional
  modules in `internal/httpapi/server.go` and `internal/httpapi/routes/`.
- Case-action queue processing in `internal/workqueue/queue.go` and `internal/workqueue/queue.go`.
- Operator-only v4 import, migration, and storage verification commands in
  `cmd/quack-v4-import`, `cmd/quack-migrate`, and `cmd/quack-storage-verify`.
- Existing local container packaging in `compose.yaml` and `Dockerfile`;
  proposed release-infrastructure changes remain explicitly unauthorized.

Relevant files:

- `cmd/quack/main.go`
- `internal/httpapi/server.go`
- `internal/httpapi/routes/router.go`
- `internal/discordbot/commands/case.go`
- `internal/discordbot/interactions/dispatcher.go`
- `internal/discordbot/ui/message.go`
- `internal/workqueue/queue.go`
- `compose.yaml`
