# Agent Guidelines

## Go documentation

- Add GoDoc-style comments to functions, methods, interfaces, and types, including unexported declarations where practical.
- Start comments for exported declarations with the declaration's exact name.
- Explain what the declaration does and why it exists, including important invariants, side effects, lifecycle behavior, or architectural boundaries.
- Avoid comments that merely restate the identifier. Keep comments accurate when behavior changes.

## User-owned tmux sessions

- Treat every existing tmux session, window, and pane as user-owned state.
- Before starting a server, watcher, log tail, or other long-running process, check whether the user already has one running in tmux when that session is available.
- Do not kill, restart, interrupt, replace, or send input to a tmux process without the user's explicit permission.
- Do not change pane layouts, active windows, session names, or tmux configuration unless specifically requested.
- Prefer non-invasive inspection. If work requires interacting with an existing tmux process, explain the intended action first and preserve the user's current session state.

## Verification

- Run `gofmt` on changed Go files.
- Run backend Go commands from `apps/backend`.
- Run the narrowest relevant tests, followed by `go test ./...` from `apps/backend` when the environment permits it.
- Preserve unrelated user changes and avoid committing generated binaries or temporary files.
