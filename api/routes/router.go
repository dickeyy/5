package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/structs"
)

func SetupRoutes(r *gin.Engine, services *app.Services) {
	r.GET("/status", func(c *gin.Context) { status(c, services) })
	r.GET("/ops/status", func(c *gin.Context) { globalOpsStatus(c, services) })
	r.GET("/guilds/:discordGuildID/ops/status", func(c *gin.Context) { guildOpsStatus(c, services) })
	setupAuthRoutes(r, services)
	setupGuildRoutes(r, services)
}

func setupGuildRoutes(r *gin.Engine, services *app.Services) {
	guilds := r.Group("/guilds")
	guilds.Use(middleware.RequireAuth(services.Store))

	guilds.GET("", func(c *gin.Context) { listUserGuilds(c, services) })
	guilds.GET("/:discordGuildID/me", middleware.RequireGuildContext(services, ""), guildMe)
	guilds.GET("/:discordGuildID/templates", middleware.RequireGuildContext(services, structs.PermissionActionCaseTemplateRead), func(c *gin.Context) {
		listTemplates(c, services)
	})
	guilds.POST("/:discordGuildID/templates", middleware.RequireGuildContext(services, structs.PermissionActionCaseTemplateWrite), func(c *gin.Context) {
		createTemplate(c, services)
	})
	guilds.GET("/:discordGuildID/templates/:templateID", middleware.RequireGuildContext(services, structs.PermissionActionCaseTemplateRead), func(c *gin.Context) {
		getTemplate(c, services)
	})
	guilds.PATCH("/:discordGuildID/templates/:templateID", middleware.RequireGuildContext(services, structs.PermissionActionCaseTemplateWrite), func(c *gin.Context) {
		updateTemplate(c, services)
	})
	guilds.DELETE("/:discordGuildID/templates/:templateID", middleware.RequireGuildContext(services, structs.PermissionActionCaseTemplateDelete), func(c *gin.Context) {
		archiveTemplate(c, services)
	})
	guilds.GET("/:discordGuildID/cases", middleware.RequireGuildContext(services, structs.PermissionActionCaseCreate), func(c *gin.Context) {
		listCases(c, services)
	})
	guilds.POST("/:discordGuildID/cases", middleware.RequireGuildContext(services, structs.PermissionActionCaseCreate), func(c *gin.Context) {
		createCase(c, services)
	})
	guilds.GET("/:discordGuildID/cases/:caseRef", middleware.RequireGuildContext(services, structs.PermissionActionCaseCreate), func(c *gin.Context) {
		getCase(c, services)
	})
	guilds.GET("/:discordGuildID/users/:targetDiscordUserID/cases", middleware.RequireGuildContext(services, structs.PermissionActionCaseCreate), func(c *gin.Context) {
		listUserCases(c, services)
	})
	guilds.GET("/:discordGuildID/audit-log", middleware.RequireGuildContext(services, structs.PermissionActionAuditRead), func(c *gin.Context) {
		listAuditLog(c, services)
	})
}
