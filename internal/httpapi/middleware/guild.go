package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

const ContextGuildKey = "guild_context"

// RequireGuildContext is a middleware function that requires a valid guild context
func RequireGuildContext(services *quack.Services, requiredAction model.PermissionAction) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := GetAuthSession(c)
		if session == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing auth session"})
			return
		}

		guildContext, err := services.Guilds.ResolveStaffContext(c.Request.Context(), session, c.Param("discordGuildID"))
		if err != nil {
			status := http.StatusForbidden
			if errors.Is(err, quack.ErrBotNotInGuild) {
				status = http.StatusNotFound
			}
			c.AbortWithStatusJSON(status, gin.H{"error": "live guild authorization unavailable"})
			return
		}

		if err := services.Guilds.Authorize(c.Request.Context(), guildContext, requiredAction, model.AuditSourceAPI); err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": quack.ErrAuthorizationDenied.Error()})
			return
		}

		c.Set(ContextGuildKey, guildContext)
		c.Next()
	}
}

// GetGuildContext retrieves the guild context from Gin context
func GetGuildContext(c *gin.Context) *quack.GuildStaffContext {
	v, ok := c.Get(ContextGuildKey)
	if !ok {
		return nil
	}

	guildContext, ok := v.(*quack.GuildStaffContext)
	if !ok {
		return nil
	}

	return guildContext
}
