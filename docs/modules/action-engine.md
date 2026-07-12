# Action Engine

The action engine in `internal/quack/actions.go` executes the selected
zero-or-one enforcement action, persists every attempt, exposes staff recovery,
and delivers one post-outcome case notification.

## Execution And Fencing

1. `ClaimNextCaseAction` serializes a case and creates a running attempt with a
   bounded lease and unique fence token.
2. A crash-left claim is eligible after lease expiry. Recovery closes the old
   running attempt as `lease_expired`, increments the attempt number, and issues
   a new token.
3. `CompleteCaseAction` accepts only the current token, so a stale worker cannot
   overwrite reclaimed work.
4. The persisted retry time makes safe retries discoverable after restart.

Outbound execution has a Quack-owned timeout. Total attempts equal the initial
attempt plus the template's safe retry count.

## Discord Executors

- `timeout_user` uses the exact configured duration.
- `kick_user` uses case-number and official-reason audit text.
- `ban_user` uses Discord's exact seconds-based history deletion value.
- `remove_timeout` and `unban_user` are explicit staff-confirmed reversals
  linked to the original execution and, when supplied, an accepted appeal.

The adapter classifies validation, permission/hierarchy, unknown resource,
rate-limit, server, network, timeout, and ambiguous outcomes into redacted
attempt state. Only failures known not to have executed are automatically
retryable. Uncertain kick, ban, and reversal outcomes remain in staff review.

## Recovery Controls

Failed actions have a stable paginated review query. Retry rechecks live actor
permission, bot permission, target membership, and both hierarchies. Dismissal
removes the item from active review without deleting history. Void delegates to
the case correction flow. Reversal validates the original succeeded action.
Controls are idempotent and append case/audit history.

## One Case Notification

Notification is separate from enforcement and validity. For kick and ban, the
worker opens the DM channel before enforcement. After the action is terminal,
it renders one bounded product-owned message with guild name, official reason,
visible context, outcome, case reference, appeal access, and optional guild
introduction/footer.

A pre-send claim can recover after a crash. Immediately before the external
send, the worker crosses a second durable fence. A crash after that point is
ambiguous and is never automatically repeated. Delivery failure is visible and
audited but never invalidates the case or causes an action retry to resend it.
