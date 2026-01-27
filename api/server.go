package api

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/api/routes"
	"github.com/quackdiscord/bot/lib"
	"github.com/rs/zerolog/log"
)

func Start() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Mount("/", routes.PublicRouter())

	log.Info().Msg("Starting API on port " + lib.Config.API.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", lib.Config.API.Port), r); err != nil {
		log.Error().Err(err).Msg("Failed to start server")
	}
}
