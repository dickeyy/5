# Event Queue

The event queue is the process-local worker pool used for asynchronous work.
Right now its only live workload is case-action execution. The implementation is
in `services/event-queue.go`.

## Structure

`EventQueue` contains:

- a buffered `chan structs.QueueEvent`
- a worker count
- a wait group for worker shutdown
- an `active` flag guarded by `mu`

Queue event shape, from `structs/queue.go`:

- `Type`: log-friendly event name
- `Data`: opaque payload
- `Handler`: function accepting `structs.DataStore` and the payload

The queue stores the shared `*storage.Store` in a package-global variable during
`Init`, and passes it to every handler.

## Lifecycle

Startup order from `main.go` matters:

1. create the storage store
2. call `EventQueue.Init(store)`
3. call `EventQueue.Start()`
4. let producers enqueue work

Behavior by method:

- `Init`: allocates the queue with `lib.Config.EventQueue.Size`
- `Start`: flips `active` and launches `workers` goroutines
- `Enqueue`: returns whether the event was accepted and records a drop counter
  if the queue is inactive or the buffer is full
- `Process`: executes the handler and recovers panics
- `Stop`: closes the channel and waits for workers to exit
- `IsActive` and `QueueSize`: read-only helpers

## Delivery Guarantees

The queue is best-effort, in-process delivery:

- no persistence
- no acknowledgement or replay log
- no blocking backpressure for producers
- no dead-letter handling

When the buffer is full, `Enqueue` logs and drops the event. Queue events carry
an event ID, creation timestamp, request ID, and correlation ID. When the
process restarts, pending actions are recovered separately by
`app.EnqueuePendingCaseActions`, which queries storage for executable cases and
re-enqueues them.

Queue stats are exposed through guarded ops status responses and include buffer
size, worker count, active state, queue depth, accepted events, dropped events,
processed events, failed events, and panics.

## Current Workload

`app/actions_queue.go` publishes `case_action_execution` events with a payload
containing `CaseID`. The handler type-asserts the shared datastore back to
`*storage.Store` and runs the action engine.

That means queue concurrency is global, but actual action ordering is still
serialized per case by `storage.ClaimNextCaseAction`.

## Maintainability Notes

- The package-global `EQ` and package-global datastore pointer make the queue
  easy to access, but they also hard-code a single process-wide queue instance.
- Handler execution is synchronous inside each worker goroutine. Long-running
  handlers consume a worker until they finish.
- Because full buffers drop work, increasing `EventQueue.Size` and worker count
  is currently the only built-in tuning path.

Relevant files:

- `services/event-queue.go`
- `structs/queue.go`
- `app/actions_queue.go`
- `main.go`
