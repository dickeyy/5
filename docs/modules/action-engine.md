# Action Engine

The action engine is the asynchronous execution path for case actions. It starts
after case creation persists `CaseActionExecution` rows and keeps pulling the
next executable action for a case until none remain. The main orchestration
lives in `app/actions.go`.

## Responsibilities

- claim the next runnable action for one case
- dispatch that action to an executor module
- persist attempts, execution status, and case events
- send optional user notifications after successful moderation actions
- reschedule retryable failures
- skip later actions when a failure should stop the pipeline

## Execution Flow

1. `CaseService.Create` persists the case, initial event, and action execution
   rows, then calls `enqueueCaseActions` in `app/actions_queue.go`.
2. The queue handler builds a fresh `ActionService` and calls
   `ProcessCaseActions(caseID)`.
3. `ProcessCaseActions` loops on `storage.Store.ClaimNextCaseAction`, which
   locks the case and next eligible execution row for that case.
4. `processClaimedAction` looks up the executor by `ActionType`, falls back to
   `actions.Unsupported`, and runs it.
5. `CompleteCaseAction` writes a `CaseActionAttempt`, updates the execution
   status, appends a case event, writes audit data, and recomputes the case
   status.
6. If the action failed and the template snapshot does not allow
   `continue_on_error`, `SkipCaseActions` marks later rows as skipped.
7. If the failure is retryable and the execution allows retries,
   `scheduleCaseActions` re-enqueues the case after `next_retry_at`.

## Executor Map

`NewActionService` wires action types to executor modules:

- `send_dm`: implemented in `app/actions/send_dm.go`
- `timeout_user`: stubbed, currently returns `action_not_implemented`
- `kick_user`: stubbed, currently returns `action_not_implemented`
- `ban_user`: stubbed, currently returns `action_not_implemented`

This means warning-level DM notifications are the only live end-to-end action
path today. Template-authored moderation actions are persisted and scheduled,
but they currently fail intentionally until the Discord moderation calls are
implemented.

## Notification Rules

There are two notification paths:

- Level notification: `CaseService.Create` inserts an internal `send_dm`
  execution when the selected level has `notify_user` enabled.
- Action notification: after an executor succeeds, `executeAction` sends a DM
  when `CaseActionExecution.NotifyUser` is true.

Action notifications happen only after executor success. Unsupported or failed
actions do not send the follow-up DM.

## Retry And Failure Semantics

- Retries require both `Result.Retryable` and `CaseActionExecution.SafeForRetry`
  to be true.
- `shouldRetryAction` currently allows retry while
  `AttemptCount <= MaxRetries`, so total attempts equal initial execution plus
  the configured retry count.
- `nextRetryTime` uses `RetryBackoffMS`, defaulting to `1000` when unset.
- `continueOnError` is read from the stored template snapshot, not live template
  rows. Existing cases keep the policy that was captured at creation time.

## Storage Contract

The action engine depends on these repository methods in `storage/cases.go`:

- `ClaimNextCaseAction`
- `CompleteCaseAction`
- `SkipCaseActions`
- `ListExecutableCaseIDs`

`ClaimNextCaseAction` also enforces per-case serialization by refusing to claim
another row while one execution for the same case is already `running`.

## Maintainability Notes

- `processCaseActionQueueEvent` constructs a new `ActionService` per queue
  event. Shared executor state does not currently exist.
- `scheduleCaseActions` uses an in-process timer goroutine. Delayed retries are
  not durable across process restarts; startup recovery relies on
  `EnqueuePendingCaseActions`.
- Because `continue_on_error` is read from `TemplateSnapshotJSON`, any change to
  snapshot shape must stay backward compatible with `continueOnError`.

Relevant files:

- `app/actions.go`
- `app/actions_queue.go`
- `app/actions/types.go`
- `app/actions/send_dm.go`
- `app/actions/timeout.go`
- `app/actions/kick.go`
- `app/actions/ban.go`
- `app/discord_actions.go`
- `storage/cases.go`
- `app/actions_test.go`
