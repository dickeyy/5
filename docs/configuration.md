# Configuration

## Runtime Dependencies

The process expects:

- MySQL, connected through `DATABASE_DSN` in `services/db.go`
- Redis, connected through `REDIS_URL` in `services/redis.go`
- Discord bot credentials and OAuth settings loaded in `lib/config.go`

The startup path fails fast on missing critical storage env vars and on failed
DB or Redis pings. See `lib/config.go`, `services/db.go`, and
`services/redis.go`.

## Environment Variables

Configured in `lib/config.go`.

| Variable | Required | Purpose |
| --- | --- | --- |
| `ENVIRONMENT` | no | Runtime mode. Defaults to `dev`. |
| `API_PORT` | no | API listen port. Defaults to `8080`. |
| `OPS_STATUS_TOKEN` | no | Enables global `GET /ops/status` when supplied and matched by `X-Quack-Ops-Key`. |
| `DATABASE_DSN` | yes | MySQL DSN for GORM. |
| `REDIS_URL` | yes | Redis connection URL. |
| `DISCORD_TOKEN` or `DEV_DISCORD_TOKEN` | yes | Discord bot token. `DEV_` override is used when `ENVIRONMENT=dev`. |
| `DISCORD_APP_ID` or `DEV_DISCORD_APP_ID` | yes | Discord application ID. |
| `DISCORD_CLIENT_SECRET` or `DEV_DISCORD_CLIENT_SECRET` | needed for OAuth | Discord OAuth client secret. |
| `DISCORD_OAUTH_REDIRECT_URI` | needed for OAuth | OAuth callback URI used by `/auth/discord/callback`. |
| `DISCORD_OAUTH_SCOPES` | no | OAuth scopes. Defaults to `identify guilds`. |
| `DISCORD_COMMAND_GUILD_ID` | no | Optional test-guild command sync target. |
| `DISCORD_COMMAND_PRUNE` | no | Enables command pruning on sync. Defaults to `false`. |
| `AUTH_SESSION_COOKIE_NAME` | no | Session cookie name. Defaults to `quack_session`. |
| `AUTH_SESSION_TTL_HOURS` | no | Session TTL in hours. Defaults to `168`. |
| `AUTH_STATE_TTL_MINUTES` | no | OAuth state TTL in minutes. Defaults to `10`. |
| `AUTH_POST_LOGIN_REDIRECT` | no | Default post-login redirect path. Defaults to `/`. |
| `AUTH_COOKIE_SECURE` | no | Secure-cookie toggle. Defaults to `true` outside `dev`. |
| `EVENT_QUEUE_SIZE` | no | In-process queue buffer size. Defaults to `1000`. |
| `EVENT_QUEUE_WORKERS` | no | Number of queue workers. Defaults to `3`. |

`.env.example` mirrors the local Compose workflow and includes the same
development-oriented defaults used by `compose.yaml`. The app service profile in
`compose.yaml` also injects container-local values for `DATABASE_DSN` and
`REDIS_URL`, so those two settings do not need to point at `127.0.0.1` when the
service runs inside Compose.

## Compose-Specific Notes

The dependency-only workflow and the full app-container workflow use the same
env names, but with different responsibilities:

- `docker compose up -d` expects your host-side `.env` to point at
  `127.0.0.1:3306` and `127.0.0.1:6379`
- `docker compose --profile app up --build` injects container-local
  `DATABASE_DSN` and `REDIS_URL` values for the app service
- Discord and auth-related env vars still need to be populated by your local
  `.env` when the app profile is enabled

In practice, the variables that matter specifically for the app container
profile are the app runtime settings and Discord/auth credentials from
`.env.example`, especially:

- `ENVIRONMENT`
- `API_PORT`
- `OPS_STATUS_TOKEN`
- `DEV_DISCORD_TOKEN`
- `DEV_DISCORD_APP_ID`
- `DEV_DISCORD_CLIENT_SECRET`
- `DISCORD_OAUTH_REDIRECT_URI`
- `DISCORD_COMMAND_GUILD_ID`
- `DISCORD_COMMAND_PRUNE`
- `AUTH_SESSION_COOKIE_NAME`
- `AUTH_SESSION_TTL_HOURS`
- `AUTH_STATE_TTL_MINUTES`
- `AUTH_POST_LOGIN_REDIRECT`
- `AUTH_COOKIE_SECURE`
- `EVENT_QUEUE_SIZE`
- `EVENT_QUEUE_WORKERS`

## Local Auth and CORS Notes

The API server only allows credentialed browser requests from
`http://localhost:3000` and `http://127.0.0.1:3000`. That list is hardcoded in
`api/server.go`.

If the dashboard runs on a different origin, update `api/server.go` before
expecting cookie-based requests to work.

## Config Loading Rules

`lib.LoadConfig()` reads `.env` through `github.com/joho/godotenv` and then
builds `lib.Config`.

When `ENVIRONMENT=dev`, some Discord values are read from `DEV_*` names first:

- `DEV_DISCORD_TOKEN`
- `DEV_DISCORD_APP_ID`
- `DEV_DISCORD_CLIENT_SECRET`

For non-dev environments, the non-prefixed names are used directly.

Relevant files:

- `lib/config.go`
- `structs/config.go`
- `.env.example`
- `compose.yaml`
- `api/server.go`
- `services/db.go`
- `services/redis.go`
