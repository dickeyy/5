package quack

import (
	"context"

	"github.com/quackdiscord/bot/internal/quack/idutil"
)

// NewTraceID creates a sortable random identifier for tracing across HTTP, Discord, and workers.
func NewTraceID() string {
	return idutil.NewTraceID()
}

// ContextWithRequestID returns a derived context carrying request id for cross-transport tracing.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return idutil.ContextWithRequestID(ctx, requestID)
}

// ContextWithCorrelationID returns a derived context carrying correlation id for cross-transport tracing.
func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return idutil.ContextWithCorrelationID(ctx, correlationID)
}

// ContextWithTrace returns a derived context carrying trace for cross-transport tracing.
func ContextWithTrace(ctx context.Context, requestID, correlationID string) context.Context {
	return idutil.ContextWithTrace(ctx, requestID, correlationID)
}

// RequestIDFromContext reads request idfrom context from context without requiring callers to know the private key type.
func RequestIDFromContext(ctx context.Context) string {
	return idutil.RequestIDFromContext(ctx)
}

// CorrelationIDFromContext reads correlation idfrom context from context without requiring callers to know the private key type.
func CorrelationIDFromContext(ctx context.Context) string {
	return idutil.CorrelationIDFromContext(ctx)
}

// TraceIDsFromContext reads trace ids from context without requiring callers to know the private key type.
func TraceIDsFromContext(ctx context.Context) (string, string) {
	return idutil.TraceIDsFromContext(ctx)
}
