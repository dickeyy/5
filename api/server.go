package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/api/routes"
	"github.com/quackdiscord/bot/lib"
	"github.com/rs/zerolog/log"
)

func Start() {
	if lib.Config.Environment != "dev" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger)

	routes.SetupRoutes(r)

	log.Info().Msg("Starting API on port " + lib.Config.API.Port)
	if err := r.Run(fmt.Sprintf(":%s", lib.Config.API.Port)); err != nil {
		log.Error().Err(err).Msg("Failed to start server")
	}
}
