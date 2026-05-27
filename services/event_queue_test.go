package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/quackdiscord/bot/lib"
	"github.com/quackdiscord/bot/structs"
)

func TestEventQueueAcceptsProcessesAndTracksTrace(t *testing.T) {
	setQueueTestConfig(t, 4, 1)
	(&EventQueue{}).Init(nil)
	EQ.Start()
	defer EQ.Stop()

	processed := make(chan struct{}, 1)
	accepted := EQ.Enqueue(structs.QueueEvent{
		Type:          "test_event",
		RequestID:     "req-1",
		CorrelationID: "corr-1",
		Handler: func(ctx context.Context, _ structs.DataStore, _ any) error {
			if lib.RequestIDFromContext(ctx) != "req-1" || lib.CorrelationIDFromContext(ctx) != "corr-1" {
				t.Errorf("trace ids were not propagated into handler context")
			}
			processed <- struct{}{}
			return nil
		},
	})
	if !accepted {
		t.Fatalf("expected event to be accepted")
	}

	select {
	case <-processed:
	case <-time.After(time.Second):
		t.Fatalf("event was not processed")
	}

	stats := EQ.Stats()
	if stats.EnqueuedTotal != 1 || stats.ProcessedTotal != 1 || stats.DroppedTotal != 0 {
		t.Fatalf("unexpected queue stats: %+v", stats)
	}
	if stats.LastProcessedType != "test_event" {
		t.Fatalf("expected last processed type, got %+v", stats)
	}
}

func TestEventQueueDropsWhenInactiveAndCountsPanics(t *testing.T) {
	setQueueTestConfig(t, 1, 0)
	(&EventQueue{}).Init(nil)

	if EQ.Enqueue(structs.QueueEvent{Type: "inactive", Handler: func(context.Context, structs.DataStore, any) error { return nil }}) {
		t.Fatalf("expected inactive queue enqueue to be rejected")
	}
	if EQ.Stats().DroppedTotal != 1 {
		t.Fatalf("expected inactive drop count, got %+v", EQ.Stats())
	}

	EQ.Process(structs.QueueEvent{
		Type: "panic",
		Handler: func(context.Context, structs.DataStore, any) error {
			panic("boom")
		},
	})
	stats := EQ.Stats()
	if stats.PanickedTotal != 1 || stats.FailedTotal != 1 {
		t.Fatalf("expected panic/failure counters, got %+v", stats)
	}

	EQ.Process(structs.QueueEvent{
		Type: "error",
		Handler: func(context.Context, structs.DataStore, any) error {
			return errors.New("failed")
		},
	})
	if EQ.Stats().FailedTotal != 2 {
		t.Fatalf("expected handler error to increment failure counter, got %+v", EQ.Stats())
	}
}

func setQueueTestConfig(t testing.TB, size, workers int) {
	t.Helper()
	previous := lib.Config
	lib.Config = &structs.Config{
		EventQueue: structs.EventQueueConfig{Size: size, Workers: workers},
	}
	t.Cleanup(func() {
		lib.Config = previous
		EQ = nil
	})
}
