# Action Engine

The action engine is the asynchronous execution path for case actions. It starts
after case creation persists `CaseActionExecution` rows and keeps pulling the
next executable action for a case until none remain. The main orchestration
lives in `internal/quack/actions.go`.

## Responsibilities

- claim the next runnable action for one case
- dispatch that action to an executor module
- persist attempts, execution status, and case events
- send optional user notifications after successful moderation actions
- reschedule retryable failures
- skip later actions when a failure should stop the pipeline

## Execution Flow

1. `CaseService.Create` persists the case, initial event, and action execution
   rows, then submits a case-ID wake-up hint through `CaseWorkScheduler`.
2. The injected queue calls the existing `ActionService.ProcessCaseActions`.
3. `ProcessCaseActions` loops on `Repository.ClaimNextCaseAction`, which
   locks the case and next eligible execution row for that case.
4. `processClaimedAction` looks up the executor by `ActionType`, falls back to
   `actions.Unsupported`, and runs it.
5. `CompleteCaseAction` writes a `CaseActionAttempt`, updates the execution
   status, appends a case event, writes audit data, and recomputes the case
   status.
6. If the action failed and the template snapshot does not allow
   `continue_on_error`, `SkipCaseActions` marks later rows as skipped.
7. If the failure is retryable and the execution allows retries,
   the persisted `next_retry_at` makes the case discoverable when due.

## Executor Map

`NewActionService` wires action types to executor modules:

- `send_dm`: implemented in `internal/quack/actionmods/send_dm.go`
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

The action engine depends on these repository methods in `internal/store/cases.go`:

- `ClaimNextCaseAction`
- `CompleteCaseAction`
- `SkipCaseActions`
- `ListExecutableCaseIDs`

`ClaimNextCaseAction` also enforces per-case serialization by refusing to claim
another row while one execution for the same case is already `running`.

## Maintainability Notes

- The queue is injected through a core-owned interface; the application core
  does not import the worker implementation.
- Delayed retries are durable because timing is stored in MySQL and discovered
  by the scheduler rather than owned by a goroutine.
- Because `continue_on_error` is read from `TemplateSnapshotJSON`, any change to
  snapshot shape must stay backward compatible with `continueOnError`.

Relevant files:

- `internal/quack/actions.go`
- `internal/workqueue/queue.go`
- `internal/quack/actionmods/types.go`
- `internal/quack/actionmods/send_dm.go`
- `internal/quack/actionmods/timeout.go`
- `internal/quack/actionmods/kick.go`
- `internal/quack/actionmods/ban.go`
- `internal/store/cases.go`
- `internal/quack/actions_test.go`
