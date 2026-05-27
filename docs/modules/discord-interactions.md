# Discord Interactions

The Discord interaction stack handles more than slash commands now. Runtime
dispatch lives in `discord/interactions/dispatcher.go`, while shared message,
embed, response, and custom-ID helpers live under `discord/ui/`.

This module is infrastructure for commands, message components, and modal
submits. The current production registration path is still command-centric:
`discord/commands/registry.go` installs the dispatcher, and `/case` is the only
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

`ComponentRegistry` in `discord/interactions/components.go` stores handlers by
`namespace:action`.

Lookup flow:

1. Decode the incoming Discord custom ID with `ui.DecodeCustomID(...)`.
2. Build the lookup key from the decoded `Namespace` and `Action`.
3. Return the registered handler for either the component or modal table.

Custom IDs are versioned and use the four-part format:

`namespace:action:version:payload`

Validation rules enforced by `discord/ui/components.go`:

- namespace, action, and version are required
- those fields cannot contain `:`
- the full encoded value must stay within Discord's 100-rune limit

## UI And Response Helpers

`discord/ui/message.go` defines the shared message and edit model used by
interaction handlers:

- `Message` for content, embeds, components, files, and allowed mentions
- `Edit` for webhook edits
- embed helpers such as `SuccessEmbed`, `ErrorEmbed`, and the fluent `Embed`
  builder

`discord/ui/responses.go` wraps Discord interaction response types:

- immediate public or ephemeral messages
- deferred public or ephemeral responses
- message updates
- autocomplete responses
- modal responses

The `/case add` command uses this layer to defer publicly and then replace the
deferred placeholder with a case-created embed from `discord/ui/views/case.go`.

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

The current interaction tests in `discord/interactions/dispatcher_test.go` and
`discord/ui/responses_test.go` cover:

- immediate command responses
- deferred async command responses
- async error fallback edits
- component and modal routing
- response helper output shape

Relevant files:

- `discord/commands/registry.go`
- `discord/interactions/dispatcher.go`
- `discord/interactions/components.go`
- `discord/ui/components.go`
- `discord/ui/message.go`
- `discord/ui/responses.go`
- `discord/ui/views/case.go`
- `discord/interactions/dispatcher_test.go`
- `discord/ui/responses_test.go`
