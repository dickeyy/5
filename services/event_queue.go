package services

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quackdiscord/bot/lib"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
	"github.com/rs/zerolog/log"
)

type EventQueue struct {
	Queue   chan structs.QueueEvent
	workers int
	wg      sync.WaitGroup
	active  bool
	mu      sync.RWMutex
	stats   EventQueueStats
}

type EventQueueStats struct {
	BufferSize        int    `json:"buffer_size"`
	Workers           int    `json:"workers"`
	Active            bool   `json:"active"`
	QueueSize         int    `json:"queue_size"`
	EnqueuedTotal     uint64 `json:"enqueued_total"`
	DroppedTotal      uint64 `json:"dropped_total"`
	ProcessedTotal    uint64 `json:"processed_total"`
	FailedTotal       uint64 `json:"failed_total"`
	PanickedTotal     uint64 `json:"panicked_total"`
	LastProcessedID   string `json:"last_processed_id,omitempty"`
	LastProcessedType string `json:"last_processed_type,omitempty"`
}

var EQ *EventQueue

var s *storage.Store

// Initializes the event queue
func (q *EventQueue) Init(st *storage.Store) {
	s = st
	EQ = &EventQueue{
		Queue:   make(chan structs.QueueEvent, lib.Config.EventQueue.Size),
		workers: lib.Config.EventQueue.Workers,
		active:  false,
	}
	log.Info().
		Int("buffer_size", lib.Config.EventQueue.Size).
		Int("workers", lib.Config.EventQueue.Workers).
		Msg("Event queue initialized")
}

// Start begins processing events from the queue with multiple workers
func (q *EventQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.active {
		log.Warn().Msg("Event queue already running")
		return
	}

	q.active = true
	log.Info().Int("workers", q.workers).Msg("Starting event queue workers")

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

// worker processes events from the queue
func (q *EventQueue) worker(id int) {
	defer q.wg.Done()

	log.Debug().Int("worker_id", id).Msg("Event queue worker started")

	for event := range q.Queue {
		q.Process(event)
	}

	log.Debug().Int("worker_id", id).Msg("Event queue worker stopped")
}

// Enqueue adds an event to the queue and reports whether it was accepted.
func (q *EventQueue) Enqueue(event structs.QueueEvent) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if !q.active {
		atomic.AddUint64(&q.stats.DroppedTotal, 1)
		log.Warn().
			Str("event_type", event.Type).
			Msg("Attempted to enqueue event but queue is not active")
		return false
	}
	if event.ID == "" {
		event.ID = lib.NewTraceID()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if event.RequestID == "" {
		event.RequestID = lib.NewTraceID()
	}
	if event.CorrelationID == "" {
		event.CorrelationID = event.RequestID
	}

	select {
	case q.Queue <- event:
		atomic.AddUint64(&q.stats.EnqueuedTotal, 1)
		return true
	default:
		atomic.AddUint64(&q.stats.DroppedTotal, 1)
		log.Warn().
			Str("event_id", event.ID).
			Str("event_type", event.Type).
			Str("correlation_id", event.CorrelationID).
			Msg("Event queue full, dropping event")
		return false
	}
}

// Process executes the event handler
func (q *EventQueue) Process(event structs.QueueEvent) {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			atomic.AddUint64(&q.stats.PanickedTotal, 1)
			atomic.AddUint64(&q.stats.FailedTotal, 1)
			log.Error().
				Str("event_id", event.ID).
				Str("event_type", event.Type).
				Str("request_id", event.RequestID).
				Str("correlation_id", event.CorrelationID).
				Interface("panic", r).
				Msg("Panic while processing event")
		}
	}()

	ctx := lib.ContextWithTrace(context.Background(), event.RequestID, event.CorrelationID)
	if event.Handler == nil {
		atomic.AddUint64(&q.stats.FailedTotal, 1)
		log.Error().
			Str("event_id", event.ID).
			Str("event_type", event.Type).
			Msg("Queue event missing handler")
		return
	}
	if err := event.Handler(ctx, s, event.Data); err != nil {
		atomic.AddUint64(&q.stats.FailedTotal, 1)
		log.Error().
			Err(err).
			Str("event_id", event.ID).
			Str("event_type", event.Type).
			Str("request_id", event.RequestID).
			Str("correlation_id", event.CorrelationID).
			Dur("duration", time.Since(start)).
			Msg("Queue event failed")
		return
	}
	atomic.AddUint64(&q.stats.ProcessedTotal, 1)
	q.mu.Lock()
	q.stats.LastProcessedID = event.ID
	q.stats.LastProcessedType = event.Type
	q.mu.Unlock()
	log.Debug().
		Str("event_id", event.ID).
		Str("event_type", event.Type).
		Str("request_id", event.RequestID).
		Str("correlation_id", event.CorrelationID).
		Dur("duration", time.Since(start)).
		Msg("Queue event processed")
}

// Stop gracefully shuts down the event queue
func (q *EventQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.active {
		log.Warn().Msg("Event queue already stopped")
		return
	}

	log.Info().Msg("Stopping event queue")
	q.active = false
	close(q.Queue)
	q.wg.Wait()
	log.Info().Msg("Event queue stopped")
}

// IsActive returns whether the queue is currently active
func (q *EventQueue) IsActive() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.active
}

// QueueSize returns the current number of events in the queue
func (q *EventQueue) QueueSize() int {
	return len(q.Queue)
}

func (q *EventQueue) Stats() EventQueueStats {
	if q == nil {
		return EventQueueStats{}
	}
	q.mu.RLock()
	stats := EventQueueStats{
		Active:            q.active,
		Workers:           q.workers,
		LastProcessedID:   q.stats.LastProcessedID,
		LastProcessedType: q.stats.LastProcessedType,
	}
	q.mu.RUnlock()

	if q.Queue != nil {
		stats.QueueSize = len(q.Queue)
		stats.BufferSize = cap(q.Queue)
	}
	stats.EnqueuedTotal = atomic.LoadUint64(&q.stats.EnqueuedTotal)
	stats.DroppedTotal = atomic.LoadUint64(&q.stats.DroppedTotal)
	stats.ProcessedTotal = atomic.LoadUint64(&q.stats.ProcessedTotal)
	stats.FailedTotal = atomic.LoadUint64(&q.stats.FailedTotal)
	stats.PanickedTotal = atomic.LoadUint64(&q.stats.PanickedTotal)
	return stats
}
