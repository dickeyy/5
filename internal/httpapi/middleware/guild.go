package middleware

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
	"github.com/rs/zerolog/log"
)

const ContextGuildKey = "guild_context"

// RequireGuildContext is a middleware function that requires a valid guild context
func RequireGuildContext(services *quack.Services, requiredAction model.PermissionAction) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := GetAuthSession(c)
		if session == nil {
			apierror.Write(c, http.StatusUnauthorized, apierror.CodeAuthentication, "authentication required")
			return
		}

		guildContext, err := services.Guilds.ResolveStaffContext(c.Request.Context(), session, c.Param("discordGuildID"))
		if err != nil {
			log.Warn().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Str("actor_discord_user_id", session.DiscordUserID).Str("discord_guild_id", c.Param("discordGuildID")).Msg("live guild authorization denied")
			status := http.StatusForbidden
			if errors.Is(err, quack.ErrBotNotInGuild) {
				status = http.StatusNotFound
			}
			if status == http.StatusNotFound {
				apierror.Write(c, status, apierror.CodeNotFound, "guild not found")
			} else {
				apierror.Write(c, status, apierror.CodeAuthorization, "live guild authorization unavailable")
			}
			return
		}

		if err := services.Guilds.Authorize(c.Request.Context(), guildContext, requiredAction, model.AuditSourceAPI); err != nil {
			log.Warn().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Str("actor_discord_user_id", session.DiscordUserID).Str("discord_guild_id", c.Param("discordGuildID")).Str("permission_action", string(requiredAction)).Msg("guild permission denied")
			apierror.Write(c, http.StatusForbidden, apierror.CodeAuthorization, "access denied")
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
