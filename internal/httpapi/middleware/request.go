package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/quack"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
	ContextRequestIDKey = "request_id"
)

// RequestContext encapsulates the request context rule so callers share one consistent package implementation.
func RequestContext(c *gin.Context) {
	requestID := c.GetHeader(RequestIDHeader)
	correlationID := c.GetHeader(CorrelationIDHeader)
	ctx := quack.ContextWithTrace(c.Request.Context(), requestID, correlationID)
	c.Request = c.Request.WithContext(ctx)

	requestID = quack.RequestIDFromContext(ctx)
	correlationID = quack.CorrelationIDFromContext(ctx)
	c.Set(ContextRequestIDKey, requestID)
	c.Header(RequestIDHeader, requestID)
	c.Header(CorrelationIDHeader, correlationID)

	c.Next()
}
