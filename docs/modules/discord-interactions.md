# Discord Interactions

The dispatcher in `apps/backend/internal/discordbot/interactions` supports commands,
autocomplete, components, and modals. The command registry installs `/case`
and the `Create moderation case` message context command.

## Case Creation

`/case add` accepts an active-template autocomplete value, a target, structured
visible context, and an optional message link. It refreshes Discord authority,
uses the shared evidence service, and keys idempotency by interaction ID.
Validation and capture errors remain on a private deferred acknowledgement.
Success deletes that acknowledgement and sends a public staff-channel followup
limited to case number, target, template, level, and action status.

The message context command derives the target from the selected message and
uses the same capture flow. When more policy/context selection is required, it
directs staff to `/case add` without exposing evidence.

## Staff Views And Controls

`/case view`, `list`, and `user` provide authorized, private browsing.
`/case failures`, `retry`, `dismiss`, `void`, and `reverse` expose the durable
recovery services. Confirmation is required for void and reversal. Detailed
context/evidence and technical failures never appear in public channel output.

## Response Safety

Command handlers use package-owned UI models and empty allowed mentions.
Immediate permission errors are private. Async panics and failures are recovered,
trace-linked in logs, and converted to safe interaction errors. The durable case
view remains the source of truth after an interaction token expires.
