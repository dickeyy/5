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
- `api/server.go`
- `services/db.go`
- `services/redis.go`
