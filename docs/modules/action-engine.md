# Action Engine

The action engine is the asynchronous execution path for case actions. It starts
after case creation persists `CaseActionExecution` rows and keeps pulling the
next executable action for a case until none remain. The main orchestration
lives in `internal/quack/actions.go`.

## Responsibilities

- claim the next runnable action for one case
- dispatch that action to an executor module
- persist attempts, execution status, and case events
- reschedule retryable failures

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
6. If the failure is retryable and the execution allows retries,
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

The selected level is the only template-owned notification decision.
`CaseService.Create` currently represents that decision as one internal
`send_dm` execution. Enforcement actions cannot configure or send an additional
notification. V5-012 owns the remaining transition to outcome-aware case-level
delivery after enforcement completes.

## Retry And Failure Semantics

- Retries require both `Result.Retryable` and `CaseActionExecution.SafeForRetry`
  to be true.
- `shouldRetryAction` currently allows retry while
  `AttemptCount <= MaxRetries`, so total attempts equal initial execution plus
  the configured retry count.
- `nextRetryTime` uses Quack's internal `RetryBackoffMS`, defaulting to `1000`
  when unset. Templates cannot configure retry timing.

## Storage Contract

The action engine depends on these repository methods in `internal/store/cases.go`:

- `ClaimNextCaseAction`
- `CompleteCaseAction`
- `ListExecutableCaseIDs`

`ClaimNextCaseAction` also enforces per-case serialization by refusing to claim
another row while one execution for the same case is already `running`.

## Maintainability Notes

- The queue is injected through a core-owned interface; the application core
  does not import the worker implementation.
- Delayed retries are durable because timing is stored in MySQL and discovered
  by the scheduler rather than owned by a goroutine.
- Retired action notification, ordering, and continuation columns remain frozen
  in compatibility storage but do not influence newly created template actions.

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
