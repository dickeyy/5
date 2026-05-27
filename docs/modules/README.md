# Module Docs

Focused maintainer notes for core runtime modules.

## Index

- `action-engine.md`: queued case-action execution, retries, notifications, and status transitions.
- `case-pipeline.md`: how template selection, case creation, snapshotting, and action rows fit together.
- `command-registry.md`: command registration, interaction dispatch, sync, and hash caching.
- `discord-interactions.md`: dispatcher flow, component and modal lookup, custom IDs, and shared UI helpers.
- `event-queue.md`: in-process queue lifecycle, worker behavior, and delivery limits.

These pages are intentionally narrower than `../architecture.md`. Use them when
you need to change one subsystem without re-reading the whole runtime overview.
