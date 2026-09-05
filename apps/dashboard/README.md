# Quack dashboard

The v5 dashboard is a client-gated TanStack Start application. Every route
checks the current Discord session through `GET /auth/me`; API requests include
credentials, and writes include the session CSRF token.

## Local development

1. Copy `.env.example` to `.env`.
2. Start the v5 backend on `http://localhost:8080`.
3. Run `bun install`, then `bun run dev`.

The backend must allow `http://localhost:3000` as an authenticated browser
origin. Its `AUTH_POST_LOGIN_REDIRECT` should also point to
`http://localhost:3000` so Discord OAuth returns to the dashboard.

## Production

Set `VITE_API_BASE_URL` to the public API origin, or leave it unset when API
paths are reverse-proxied on `dashboard.quack.bot`. Cross-origin deployments
must configure the backend's allowed dashboard origin, secure cookie behavior,
and `AUTH_POST_LOGIN_REDIRECT` for the dashboard origin together.

The app intentionally disables route SSR. The API session cookie can be
host-only to the API domain, so authentication is resolved in the browser and
the backend remains the authorization boundary.

## Member appeal contract

Discord case notifications link to
`/guilds/{internalGuildId}/cases/{caseId}/appeal`. The dashboard reserves that
path outside the staff shell and can render the privacy-safe member case,
existing appeal status, and requested-information form.

Initial appeal submission still depends on a backend contract addition. The
member case response currently returns only `appealable`; it does not return
the configured appeal questions needed to construct
`AppealSubmissionInput.answers`. The dashboard deliberately does not guess
default question IDs or call the staff-only appeal settings endpoint.

## Checks

```sh
bun run generate-routes
bun run test
bun run typecheck
bun run build
```
