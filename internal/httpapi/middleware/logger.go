package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/rs/zerolog/log"
)

// Logger encapsulates the logger rule so callers share one consistent package implementation.
func Logger(c *gin.Context) {
	start := time.Now()
	path := c.Request.URL.Path
	raw := c.Request.URL.RawQuery

	c.Next()

	if raw != "" {
		path = path + "?" + raw
	}

	event := log.Info()
	if len(c.Errors) > 0 {
		event = log.Error().Err(c.Errors.Last())
	}

	event.
		Str("request_id", quack.RequestIDFromContext(c.Request.Context())).
		Str("correlation_id", quack.CorrelationIDFromContext(c.Request.Context())).
		Str("method", c.Request.Method).
		Str("path", path).
		Int("status", c.Writer.Status()).
		Dur("latency", time.Since(start)).
		Msg("Request completed")
}
