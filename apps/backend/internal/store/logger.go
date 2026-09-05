package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// databaseLogger routes GORM diagnostics through slog without emitting SQL,
// bound values, or driver error text, which can contain moderation content.
type databaseLogger struct{ level logger.LogLevel }

// LogMode returns a copy so per-session verbosity never changes other callers.
func (l databaseLogger) LogMode(level logger.LogLevel) logger.Interface {
	l.level = level
	return l
}

// Info records database lifecycle events without interpolating driver arguments.
func (l databaseLogger) Info(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Info {
		slog.DebugContext(ctx, "Database event")
	}
}

// Warn records a database warning without exposing driver arguments.
func (l databaseLogger) Warn(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Warn {
		slog.WarnContext(ctx, "Database warning")
	}
}

// Error records a database failure without exposing driver arguments.
func (l databaseLogger) Error(ctx context.Context, _ string, _ ...interface{}) {
	if l.level >= logger.Error {
		slog.ErrorContext(ctx, "Database operation failed")
	}
}

// Trace reports failed and slow queries. Missing rows are expected lookups;
// normal successful queries are visible only when debug logging is enabled.
func (l databaseLogger) Trace(ctx context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.level == logger.Silent || errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	elapsed := time.Since(begin)
	level := slog.LevelDebug
	switch {
	case err != nil && l.level >= logger.Error:
		level = slog.LevelError
	case elapsed >= 200*time.Millisecond && l.level >= logger.Warn:
		level = slog.LevelWarn
	case l.level < logger.Info:
		return
	}
	if !slog.Default().Enabled(ctx, level) {
		return
	}
	sql, rows := query()
	operation := "query"
	if words := strings.Fields(sql); len(words) > 0 {
		switch first := strings.ToUpper(words[0]); first {
		case "SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP":
			operation = first
		}
	}
	slog.Log(ctx, level, "Database query completed", "operation", operation,
		"duration", elapsed, "rows", rows, "error_type", fmt.Sprintf("%T", err))
}
