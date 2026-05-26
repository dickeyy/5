# Development

## Local Workflow

The default process starts everything in one binary: API, Discord session, DB
migrations, Redis-backed storage, and the in-process action queue. See
`main.go`.

Typical loop:

1. Set the required env vars in `.env`.
2. Start MySQL and Redis with `docker compose up -d`.
3. Run the app with `go run .`.
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

## Common Commands

Start the app:

```sh
go run .
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

When you need a stable local cache path on macOS, this repo has previously been
run with:

```sh
GOCACHE=/tmp/quack-go-build-cache go test ./...
```

That cache override is a local convenience, not a code requirement.

## Where To Change Things

- Add or change API endpoints in `api/routes/` and `api/middleware/`.
- Add or change business rules in `app/`.
- Add or change persistence behavior in `storage/`.
- Add or change schema records and enums in `structs/schema.go`.
- Add or change Discord command behavior in `discord/commands/`.
- Add or change process-level infrastructure in `services/`.

The main shared service boundary is `app.Services` in `app/app.go`. Prefer
putting business behavior there rather than duplicating it in route handlers or
Discord commands.

## Current Maintainability Notes

- The `Legacy/` tree is still present but separate from the v5 runtime. The
  current process entrypoint is `main.go`, not `Legacy/main.go`.
- The dashboard-facing product handoff lives in `website-agent-plan.md`.
- The product roadmap and policy model live in `v5.md`.
- CORS is currently fixed to localhost port `3000`, which matters whenever the
  dashboard moves ports.
- Action execution is in-process, not an external worker service.

Relevant files:

- `main.go`
- `app/app.go`
- `api/routes/router.go`
- `discord/commands/case.go`
- `website-agent-plan.md`
- `v5.md`
