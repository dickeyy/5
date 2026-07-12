package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/routes"
	"github.com/quackdiscord/bot/internal/moduleintegration"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/rs/zerolog/log"
)

// Run serves the configured HTTP API until the context is canceled, then performs a bounded graceful shutdown.
func Run(ctx context.Context, cfg config.Config, services *quack.Services, moduleRuntime *moduleintegration.Runtime, discord routes.DiscordStatusProvider) error {
	if cfg.Environment != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}

	registrar, err := NewPlatformRegistrarWithRepository(cfg, services.Store)
	if err != nil {
		return fmt.Errorf("validate HTTP platform configuration: %w", err)
	}

	r := gin.New()
	if err := registrar.Register(r); err != nil {
		return fmt.Errorf("configure trusted HTTP proxies: %w", err)
	}

	if err := routes.SetupRoutesWithModules(r, services, moduleRuntime, discord); err != nil {
		return fmt.Errorf("register HTTP routes: %w", err)
	}

	log.Info().Msg("Starting API on port " + cfg.API.Port)
	server := newHTTPServer(cfg, r)
	shutdownResult := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.API.ShutdownTimeoutSeconds)*time.Second)
		defer cancel()
		err := server.Shutdown(shutdownCtx)
		if err != nil {
			log.Error().Err(err).Msg("Failed to gracefully shut down API")
		}
		shutdownResult <- err
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	if ctx.Err() != nil {
		if err := <-shutdownResult; err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
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
