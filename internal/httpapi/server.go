package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/routes"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/rs/zerolog/log"
)

// Run serves the configured HTTP API until the context is canceled, then performs a bounded graceful shutdown.
func Run(ctx context.Context, cfg config.Config, services *quack.Services, discord routes.DiscordStatusProvider) error {
	if cfg.Environment != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}

	registrar, err := NewPlatformRegistrar(cfg)
	if err != nil {
		return fmt.Errorf("validate HTTP platform configuration: %w", err)
	}

	r := gin.New()
	registrar.Register(r)

	routes.SetupRoutes(r, services, discord)

	log.Info().Msg("Starting API on port " + cfg.API.Port)
	server := newHTTPServer(cfg, r)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Failed to gracefully shut down API")
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// newHTTPServer constructs the bounded standard-library server used by Run.
func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%s", cfg.API.Port),
		Handler:           handler,
		ReadHeaderTimeout: time.Duration(cfg.API.ReadHeaderTimeoutSeconds) * time.Second,
		ReadTimeout:       time.Duration(cfg.API.ReadTimeoutSeconds) * time.Second,
		WriteTimeout:      time.Duration(cfg.API.WriteTimeoutSeconds) * time.Second,
		IdleTimeout:       time.Duration(cfg.API.IdleTimeoutSeconds) * time.Second,
	}
}
