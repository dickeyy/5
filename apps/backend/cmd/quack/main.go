package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/lmittmann/tint"
	quackruntime "github.com/quackdiscord/bot/internal/runtime"
)

// @title Quack HTTP API
// @version 5.0
// @description HTTP boundary exposed by the Quack v5 backend.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey CookieAuth
// @in header
// @name Cookie
// @securityDefinitions.apikey MetricsKey
// @in header
// @name X-Quack-Metrics-Key
// @securityDefinitions.apikey OpsKey
// @in header
// @name X-Quack-Ops-Key

// main runs Quack and converts process signals into a graceful application shutdown.
func main() {
	slog.SetDefault(slog.New(tint.NewTextHandler(os.Stderr, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := quackruntime.Run(ctx); err != nil {
		slog.Error("Quack stopped", "error", err)
		os.Exit(1)
	}
}
