package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/app"
)

const (
	RequestIDHeader     = "X-Request-ID"
	CorrelationIDHeader = "X-Correlation-ID"
	ContextRequestIDKey = "request_id"
)

func RequestContext(c *gin.Context) {
	requestID := c.GetHeader(RequestIDHeader)
	correlationID := c.GetHeader(CorrelationIDHeader)
	ctx := app.ContextWithTrace(c.Request.Context(), requestID, correlationID)
	c.Request = c.Request.WithContext(ctx)

	requestID = app.RequestIDFromContext(ctx)
	correlationID = app.CorrelationIDFromContext(ctx)
	c.Set(ContextRequestIDKey, requestID)
	c.Header(RequestIDHeader, requestID)
	c.Header(CorrelationIDHeader, correlationID)

	c.Next()
}
