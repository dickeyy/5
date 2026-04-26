package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/app"
)

func SetupRoutes(r *gin.Engine, services *app.Services) {
	r.GET("/status", func(c *gin.Context) { status(c, services) })
	setupAuthRoutes(r, services)
	setupGuildRoutes(r, services)
}

func setupGuildRoutes(r *gin.Engine, services *app.Services) {
	guilds := r.Group("/guilds")
	guilds.Use(middleware.RequireAuth(services.Store))
}
