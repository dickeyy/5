package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	quackruntime "github.com/quackdiscord/bot/internal/runtime"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Caller().Logger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := quackruntime.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("Quack stopped")
	}
}
