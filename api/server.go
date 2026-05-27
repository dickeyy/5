package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/api/routes"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/lib"
	"github.com/quackdiscord/bot/storage"
	"github.com/rs/zerolog/log"
)

func Start(s *storage.Store) {
	StartWithContext(context.Background(), s)
}

func StartWithContext(ctx context.Context, s *storage.Store) {
	if lib.Config.Environment != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}

	allowedOrigins := map[string]struct{}{
		"http://localhost:3000": {},
		"http://127.0.0.1:3000": {},
	}

	r := gin.New()
	r.Use(middleware.RequestContext)
	r.Use(gin.Recovery())
	r.Use(middleware.Logger)
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if _, ok := allowedOrigins[origin]; ok {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Correlation-ID, X-Quack-Ops-Key")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	routes.SetupRoutes(r, app.New(s))

	log.Info().Msg("Starting API on port " + lib.Config.API.Port)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", lib.Config.API.Port),
		Handler: r,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error().Err(err).Msg("Failed to gracefully shut down API")
		}
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Failed to start server")
	}
}
