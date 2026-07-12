# Discord Interactions

The Discord interaction stack handles more than slash commands now. Runtime
dispatch lives in `internal/discordbot/interactions/dispatcher.go`, while shared message,
embed, response, and custom-ID helpers live under `internal/discordbot/ui/`.

This module is infrastructure for commands, message components, and modal
submits. The current production registration path is still command-centric:
`internal/discordbot/commands/registry.go` installs the dispatcher, and `/case` is the only
registered command today.

## Dispatch Model

`Dispatcher.Handle(...)` switches on Discord interaction type:

- application commands
- autocomplete interactions
- message components
- modal submits

Command handlers are looked up through the command registry interface.
Component and modal handlers are looked up through `ComponentRegistry`.

If a handler returns an immediate response, the dispatcher sends it through
`InteractionRespond`. If the handler returns an async task, the dispatcher first
sends the deferred response, then runs the task in a goroutine and edits the
original interaction response through the responder abstraction.

## Component And Modal Registration

`ComponentRegistry` in `internal/discordbot/interactions/components.go` stores handlers by
`namespace:action`.

Lookup flow:

1. Decode the incoming Discord custom ID with `ui.DecodeCustomID(...)`.
2. Build the lookup key from the decoded `Namespace` and `Action`.
3. Return the registered handler for either the component or modal table.

Custom IDs are versioned and use the four-part format:

`namespace:action:version:payload`

Validation rules enforced by `internal/discordbot/ui/components.go`:

- namespace, action, and version are required
- those fields cannot contain `:`
- the full encoded value must stay within Discord's 100-rune limit

## UI And Response Helpers

`internal/discordbot/ui/message.go` defines the shared message and edit model used by
interaction handlers:

- `Message` for content, embeds, components, files, and allowed mentions
- `Edit` for webhook edits
- embed helpers such as `SuccessEmbed`, `ErrorEmbed`, and the fluent `Embed`
  builder

`internal/discordbot/ui/responses.go` wraps Discord interaction response types:

- immediate public or ephemeral messages
- deferred public or ephemeral responses
- message updates
- autocomplete responses
- modal responses

The `/case add` command performs live shared authorization before deferring.
Permission and membership denials therefore remain immediate and private. An
authorized request defers publicly and then replaces the placeholder with a
case-created embed from `internal/discordbot/ui/views/case.go`.

## Error And Panic Behavior

The dispatcher treats handler failures as transport-safe UI errors:

- command, component, or modal panics are recovered and logged
- recovered panics return a standard error response or edit
- async task errors are logged and converted into an edit of the original
  interaction message
- missing or invalid component and modal handlers return a standard ephemeral
  error response

This keeps command modules focused on business logic rather than Discord
transport edge cases.

## Test Coverage

The current interaction tests in `internal/discordbot/interactions/dispatcher_test.go` and
`internal/discordbot/ui/responses_test.go` cover:

- immediate command responses
- deferred async command responses
- async error fallback edits
- component and modal routing
- response helper output shape

Relevant files:

- `internal/discordbot/commands/registry.go`
- `internal/discordbot/interactions/dispatcher.go`
- `internal/discordbot/interactions/components.go`
- `internal/discordbot/ui/components.go`
- `internal/discordbot/ui/message.go`
- `internal/discordbot/ui/responses.go`
- `internal/discordbot/ui/views/case.go`
- `internal/discordbot/interactions/dispatcher_test.go`
- `internal/discordbot/ui/responses_test.go`
