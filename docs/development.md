# Development

## Local Workflow

The default process starts everything in one binary: API, Discord session, DB
migrations, Redis-backed storage, and the in-process action queue. See
`cmd/quack/main.go`.

Typical loop:

1. Set the required env vars in `.env`.
2. Start MySQL and Redis with `docker compose up -d`.
3. Run the app with `go run ./cmd/quack`.
4. Exercise API routes on `http://localhost:8080` unless `API_PORT` is changed.

The Compose defaults expose:

- MySQL on `127.0.0.1:3306`
- Redis on `127.0.0.1:6379`

Copy `.env.example` to `.env` for local defaults, then fill the Discord values.
The local dependency DSNs are:

```sh
DATABASE_DSN='quack:quack@tcp(127.0.0.1:3306)/quack?charset=utf8mb4&parseTime=True&loc=Local'
REDIS_URL='redis://127.0.0.1:6379/0'
```

To run the app in Docker as well:

```sh
docker compose --profile app up --build
```

The `app` profile uses the internal container hostnames `mysql` and `redis` and
waits for both health checks before starting.

There is no Makefile or task runner in this checkout. The repo is driven
directly through `go` commands and environment variables.

## Docker Assets

The container assets are intentionally small:

- `compose.yaml` defines the default local MySQL and Redis services
- `Dockerfile` builds a single static `quack` binary into an Alpine runtime
  image
- `.env.example` provides the env names expected by the Compose workflow

Use `docker compose up -d` when you only want the local data services and plan
to run `go run ./cmd/quack` on the host.

Use `docker compose --profile app up --build` when you want Compose to run the
Quack process as well. In that mode, the app container uses `mysql` and `redis`
service hostnames instead of `127.0.0.1`.

## Common Commands

Start the app:

```sh
go run ./cmd/quack
```

Start dependencies:

```sh
docker compose up -d
```

Stop dependencies:

```sh
docker compose down
```

Run the test suite:

```sh
go test ./...
```

Apply only database migrations without starting the other adapters:

```sh
go run ./cmd/quack-migrate up
```

See [`migrations.md`](migrations.md) before any production forward or rollback
operation.

When you need a stable local cache path on macOS, this repo has previously been
run with:

```sh
GOCACHE=/tmp/quack-go-build-cache go test ./...
```

That cache override is a local convenience, not a code requirement.

## Where To Change Things

- Add or change API endpoints in `internal/httpapi/routes/` and `internal/httpapi/middleware/`.
- Add or change business rules in `internal/quack/`.
- Add or change persistence behavior in `internal/store/`.
- Add or change schema records and enums in `internal/quack/model/schema.go`.
- Add or change Discord command behavior in `internal/discordbot/commands/`.
- Add or change process-level infrastructure in `internal/runtime/` and `internal/workqueue/`.

The main shared service boundary is `quack.Services` in `internal/quack/app.go`. Prefer
putting business behavior there rather than duplicating it in route handlers or
Discord commands.

## Current Maintainability Notes

- The `Legacy/` tree is still present but separate from the v5 runtime. The
  current process entrypoint is `cmd/quack/main.go`, not `Legacy/main.go`.
- The authoritative product definition lives in `v5.md`.
- High-level differences between that definition and the current backend live
  in `docs/v5-scope-drift.md`.
- CORS is currently fixed to localhost port `3000`, which matters whenever the
  dashboard moves ports.
- Action execution is in-process, not an external worker service.

Relevant files:

- `cmd/quack/main.go`
- `internal/quack/app.go`
- `internal/httpapi/routes/router.go`
- `internal/discordbot/commands/case.go`
- `compose.yaml`
- `Dockerfile`
- `.env.example`
- `v5.md`
- `docs/v5-scope-drift.md`
