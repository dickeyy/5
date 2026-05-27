package lib

import (
	"context"
	"strings"
	"unicode"
)

type traceContextKey string

const (
	requestIDContextKey     traceContextKey = "request_id"
	correlationIDContextKey traceContextKey = "correlation_id"
)

func NewTraceID() string {
	id, err := NewULID()
	if err != nil {
		return ""
	}
	return id
}

func ContextWithTrace(ctx context.Context, requestID, correlationID string) context.Context {
	ctx = ContextWithRequestID(ctx, requestID)
	if NormalizeTraceID(correlationID) == "" {
		correlationID = RequestIDFromContext(ctx)
	}
	return ContextWithCorrelationID(ctx, correlationID)
}

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	requestID = NormalizeTraceID(requestID)
	if requestID == "" {
		requestID = NewTraceID()
	}
	return context.WithValue(ctx, requestIDContextKey, requestID)
}

func ContextWithCorrelationID(ctx context.Context, correlationID string) context.Context {
	correlationID = NormalizeTraceID(correlationID)
	if correlationID == "" {
		correlationID = NewTraceID()
	}
	return context.WithValue(ctx, correlationIDContextKey, correlationID)
}

func RequestIDFromContext(ctx context.Context) string {
	return traceIDFromContext(ctx, requestIDContextKey)
}

func CorrelationIDFromContext(ctx context.Context) string {
	return traceIDFromContext(ctx, correlationIDContextKey)
}

func TraceIDsFromContext(ctx context.Context) (string, string) {
	requestID := RequestIDFromContext(ctx)
	correlationID := CorrelationIDFromContext(ctx)
	if correlationID == "" {
		correlationID = requestID
	}
	return requestID, correlationID
}

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

func traceIDFromContext(ctx context.Context, key traceContextKey) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(key).(string)
	return NormalizeTraceID(value)
}
