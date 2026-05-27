package app

import (
	"context"
	"time"

	"github.com/quackdiscord/bot/services"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
	"github.com/rs/zerolog/log"
)

const actionQueueEventType = "case_action_execution"

type actionQueuePayload struct {
	CaseID string
}

func enqueueCaseActions(ctx context.Context, caseID string) bool {
	if services.EQ == nil || !services.EQ.IsActive() || caseID == "" {
		return false
	}
	requestID, correlationID := TraceIDsFromContext(ctx)

	return services.EQ.Enqueue(structs.QueueEvent{
		Type:          actionQueueEventType,
		RequestID:     requestID,
		CorrelationID: correlationID,
		Data:          actionQueuePayload{CaseID: caseID},
		Handler:       processCaseActionQueueEvent,
	})
}

func scheduleCaseActions(ctx context.Context, caseID string, delay time.Duration) {
	if delay <= 0 {
		enqueueCaseActions(ctx, caseID)
		return
	}

	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		<-timer.C
		enqueueCaseActions(ctx, caseID)
	}()
}

func EnqueuePendingCaseActions(ctx context.Context, store *storage.Store, limit int) (int, error) {
	caseIDs, err := store.ListExecutableCaseIDs(ctx, limit)
	if err != nil {
		return 0, err
	}
	accepted := 0
	for _, caseID := range caseIDs {
		if enqueueCaseActions(ctx, caseID) {
			accepted++
		}
	}
	return accepted, nil
}

func processCaseActionQueueEvent(ctx context.Context, dataStore structs.DataStore, data any) error {
	store, ok := dataStore.(*storage.Store)
	if !ok || store == nil {
		log.Error().Msg("Action queue received unsupported datastore")
		return nil
	}

	payload, ok := data.(actionQueuePayload)
	if !ok || payload.CaseID == "" {
		log.Error().Interface("payload", data).Msg("Action queue received invalid payload")
		return nil
	}

	if err := NewActionService(store, NewDiscordActionClient()).ProcessCaseActions(ctx, payload.CaseID); err != nil {
		return err
	}
	return nil
}
