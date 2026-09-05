package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/quack"
)

// Logger records one structured result per request. Route patterns exclude
// user-supplied paths and query parameters, including OAuth credentials.
func Logger(c *gin.Context) {
	start := time.Now()
	c.Next()
	level := slog.LevelInfo
	if c.Writer.Status() >= 500 {
		level = slog.LevelError
	} else if c.Writer.Status() >= 400 {
		level = slog.LevelWarn
	}
	path := c.FullPath()
	if path == "" {
		path = "unmatched"
	}
	slog.Log(c.Request.Context(), level, "HTTP request completed",
		"request_id", quack.RequestIDFromContext(c.Request.Context()),
		"correlation_id", quack.CorrelationIDFromContext(c.Request.Context()),
		"method", c.Request.Method, "route", path,
		"status", c.Writer.Status(), "duration", time.Since(start))
}
