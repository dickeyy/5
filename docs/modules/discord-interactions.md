# Discord Interactions

The dispatcher in `apps/backend/internal/discordbot/interactions` supports commands,
autocomplete, components, and modals. The command registry installs `/case`
and the `Create moderation case` message context command.

## Case Creation

`/case add` accepts an active-template autocomplete value, a target, structured
visible context, and an optional message link. It refreshes Discord authority,
uses the shared evidence service, and keys idempotency by interaction ID.
Validation and capture errors remain on a private deferred acknowledgement.
Success completes the private acknowledgement, posts a permanent text response in
the invoking channel, and then removes only the acknowledgement. Completing it
first prevents Discord from reusing the private original for the first followup.
The result is limited to case number, target, template, level, and action status.

The message context command derives the target from the selected message and
uses the same capture flow. When more policy/context selection is required, it
directs staff to `/case add` without exposing evidence.

## Staff Views And Controls

`/case view`, `list`, and `user` provide authorized browsing with permanent channel messages.
These views, including case context and evidence, are visible to everyone who can
read the invoking channel.
`/case failures`, `retry`, `dismiss`, `void`, and `reverse` expose the durable
recovery services. Confirmation is required for void and reversal. Successful recovery confirmations are permanent text messages.
Creation summaries omit detailed context/evidence; staff detail views include it.

## Response Safety

Command handlers use package-owned UI models and empty allowed mentions.
Permission and operation errors are private. Component errors leave the shared
message intact. Template selection and unfinished context forms remain private
text controls. Async panics and failures are recovered,
trace-linked in logs, and converted to safe interaction errors. The durable case
view remains the source of truth after an interaction token expires.
