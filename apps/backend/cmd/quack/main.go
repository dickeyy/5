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

// main runs Quack and converts process signals into a graceful application shutdown.
func main() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Caller().Logger()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := quackruntime.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("Quack stopped")
	}
}
