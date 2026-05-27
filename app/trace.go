package app

import (
	"context"

	"github.com/quackdiscord/bot/lib"
)

func NewTraceID() string {
	return lib.NewTraceID()
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return lib.ContextWithRequestID(ctx, requestID)
}

func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return lib.ContextWithCorrelationID(ctx, correlationID)
}

func ContextWithTrace(ctx context.Context, requestID, correlationID string) context.Context {
	return lib.ContextWithTrace(ctx, requestID, correlationID)
}

func RequestIDFromContext(ctx context.Context) string {
	return lib.RequestIDFromContext(ctx)
}

func CorrelationIDFromContext(ctx context.Context) string {
	return lib.CorrelationIDFromContext(ctx)
}

func TraceIDsFromContext(ctx context.Context) (string, string) {
	return lib.TraceIDsFromContext(ctx)
}
