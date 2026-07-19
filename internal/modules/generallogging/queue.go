package generallogging

import (
	"context"
	"errors"
	"sync"
)

// ErrQueueFull reports deliberate general-log shedding under gateway pressure.
var ErrQueueFull = errors.New("general logging queue is full")

// DeliveryQueue is a bounded concurrent worker queue isolated from moderation execution.
type DeliveryQueue struct {
	service *Service
	events  chan Event
	mu      sync.RWMutex
	closed  bool
	wg      sync.WaitGroup
}

// NewDeliveryQueue starts bounded workers whose failures remain visible through module status.
func NewDeliveryQueue(ctx context.Context, service *Service, capacity, workers int) *DeliveryQueue {
	if capacity < 1 {
		capacity = 1000
	}
	if workers < 1 {
		workers = 1
	}
	queue := &DeliveryQueue{service: service, events: make(chan Event, capacity)}
	for range workers {
		queue.wg.Add(1)
		go func() {
			defer queue.wg.Done()
			for event := range queue.events {
				_ = service.Handle(ctx, event)
			}
		}()
	}
	return queue
}

// Submit enqueues without blocking Discord gateway handling and sheds when full.
func (q *DeliveryQueue) Submit(event Event) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return errors.New("general logging queue is closed")
	}
	select {
	case q.events <- cloneEvent(event):
		return nil
	default:
		return ErrQueueFull
	}
}

func cloneEvent(event Event) Event {
	event.Attachments = append([]AttachmentMetadata(nil), event.Attachments...)
	event.EmbedTypes = append([]string(nil), event.EmbedTypes...)
	if event.Metadata != nil {
		metadata := make(map[string]string, len(event.Metadata))
		for key, value := range event.Metadata {
			metadata[key] = value
		}
		event.Metadata = metadata
	}
	return event
}

// Close drains accepted events and waits for workers without affecting other module lifecycles.
func (q *DeliveryQueue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.events)
	q.mu.Unlock()
	q.wg.Wait()
}
