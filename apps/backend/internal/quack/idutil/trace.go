package idutil

import (
	"context"
	"strings"
	"unicode"
)

// traceContextKey groups the trace context key state used to keep this package's responsibilities explicit.
type traceContextKey string

const (
	requestIDContextKey     traceContextKey = "request_id"
	correlationIDContextKey traceContextKey = "correlation_id"
)

// NewTraceID creates a sortable random identifier for tracing across HTTP, Discord, and workers.
func NewTraceID() string {
	id, err := NewULID()
	if err != nil {
		return ""
	}
	return id
}

// ContextWithTrace returns a derived context carrying trace for cross-transport tracing.
func ContextWithTrace(ctx context.Context, requestID, correlationID string) context.Context {
	ctx = ContextWithRequestID(ctx, requestID)
	if NormalizeTraceID(correlationID) == "" {
		correlationID = RequestIDFromContext(ctx)
	}
	return ContextWithCorrelationID(ctx, correlationID)
}

// ContextWithRequestID returns a derived context carrying request id for cross-transport tracing.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	requestID = NormalizeTraceID(requestID)
	if requestID == "" {
		requestID = NewTraceID()
	}
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

// ContextWithCorrelationID returns a derived context carrying correlation id for cross-transport tracing.
func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	correlationID = NormalizeTraceID(correlationID)
	if correlationID == "" {
		correlationID = NewTraceID()
	}
	return context.WithValue(ctx, correlationIDContextKey, correlationID)
}

// RequestIDFromContext reads request idfrom context from context without requiring callers to know the private key type.
func RequestIDFromContext(ctx context.Context) string {
	return traceIDFromContext(ctx, requestIDContextKey)
}

// CorrelationIDFromContext reads correlation idfrom context from context without requiring callers to know the private key type.
func CorrelationIDFromContext(ctx context.Context) string {
	return traceIDFromContext(ctx, correlationIDContextKey)
}

// TraceIDsFromContext reads trace ids from context without requiring callers to know the private key type.
func TraceIDsFromContext(ctx context.Context) (string, string) {
	requestID := RequestIDFromContext(ctx)
	correlationID := CorrelationIDFromContext(ctx)
	if correlationID == "" {
		correlationID = requestID
	}
	return requestID, correlationID
}

// NormalizeTraceID accepts bounded letters, digits, and safe separators for trace propagation.
func NormalizeTraceID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == ':' {
			continue
		}
		return ""
	}
	return value
}

// traceIDFromContext reads trace idfrom context from context without requiring callers to know the private key type.
func traceIDFromContext(ctx context.Context, key traceContextKey) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return NormalizeTraceID(value)
}
