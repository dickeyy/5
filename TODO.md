# TODO

## Discord Actions

- Implement real Discord API execution for `timeout_user`.
- Implement real Discord API execution for `kick_user`.
- Implement real Discord API execution for `ban_user`.
- Add tests for real timeout, kick, and ban Discord action behavior with mocked Discord responses.
- Add rate-limit handling tests for Discord API failures.
- Add permission and hierarchy checks for real moderation actions before executing timeout, kick, or ban.
- Add dashboard warnings when a configured action type is not executable yet.

## Guild Settings

- Add guild settings storage and API endpoints.
- Add guild-wide mod-log channel configuration.
- Add Discord mod-log mirroring from audit/case events.
- Add guild-wide notification settings.
- Add bot customization settings for guilds.

## Cases

- Add case edit endpoints.
- Add case void endpoints.
- Add audit entries for case edits and case voids.
- Add dashboard API support for member-facing case views.
- Add member-facing dashboard permissions and route behavior.
- Add private staff-only case notes.
- Add public/member-visible case notes if needed.
- Add support for Discord source message links in `/case add` when available.
- Add attachment or evidence metadata support for cases.
- Add case search/filter improvements beyond current basic filters.

## Templates

- Add template context field definitions.
- Add validation for required template context fields during case creation.
- Add Discord prompts or modal flow for required template context fields.
- Add reason override policy so templates can control whether moderators may override reasons.
- Add template archive/restore behavior if archived templates need to be reactivated.

## Discord Commands

- Add Discord case lookup command.
- Add Discord case list/history command.
- Add Discord target-user case history command.
- Add Discord staff review helper commands.
- Add pagination helpers for Discord case/history views.
- Add Discord appeal commands.
- Add Discord ticket commands.

## Appeals

- Add appeal workflow APIs.
- Add appeal event/timeline behavior.
- Add configurable appeal forms per guild or template.
- Add staff appeal review APIs.
- Add member appeal submission APIs.
- Add dashboard appeal management support.

## Tickets

- Add ticket workflow APIs.
- Add ticket event/timeline behavior.
- Add configurable ticket settings.
- Add member ticket creation APIs.
- Add staff ticket review and resolution APIs.
- Add dashboard ticket management support.

## Migration And Compatibility

- Add v4/v5 import tooling if historical v4 data needs to move into v5.
- Add a dry-run import report for legacy v4 data.
- Add a v4-to-v5 case/template mapping document before any real import.
- Replace or supplement `AutoMigrate` with managed production migrations.

## Queue And Reliability

- Add durable delayed retry handling for action retries.
- Add dead-letter or replay handling for dropped/failed queue work.
- Add a persistent queue or external worker option if in-process queue limits become a problem.
- Add ops visibility for delayed retry timers that are only in memory.
- Add tests for graceful shutdown behavior.

## API And Observability

- Add generated or maintained API contract documentation for dashboard consumers.
- Add structured error response shape across all API routes.
- Add request/correlation ID fields to API error responses.
- Add metrics or logs for API auth failures and permission denials.
- Add operational guidance for rotating `OPS_STATUS_TOKEN`.

## Testing And Deployment

- Add MySQL-backed integration tests for schema, locking, JSON behavior, and transaction isolation.
- Add smoke tests for Docker Compose app startup.
- Add smoke tests for `/status` and guarded ops status in a running app container.
- Add production deployment documentation.
- Add backup and restore documentation for MySQL and Redis state.

## Notifications

- Add user-facing dashboard links in DM notifications.
- Add configurable DM notification text beyond the current default behavior.

## Docs

- Update the dashboard handoff doc now that case list, detail, history, audit, ops status, and trace behavior exist.
- Remove stale roadmap notes from docs when completed features make them inaccurate.
