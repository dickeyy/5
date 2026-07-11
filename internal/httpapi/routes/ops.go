package routes

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
)

const opsKeyHeader = "X-Quack-Ops-Key"

// globalOpsStatus encapsulates the global ops status rule so callers share one consistent package implementation.
func globalOpsStatus(c *gin.Context, services *quack.Services) {
	if !validOpsKey(c, services) {
		if strings.TrimSpace(services.Config.API.OpsStatusToken) == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "ops status is disabled"})
			return
		}
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid ops key"})
		return
	}

	status, err := services.Ops.GlobalStatus(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ops status failed"})
		return
	}
	c.JSON(http.StatusOK, status)
}

// guildOpsStatus encapsulates the guild ops status rule so callers share one consistent package implementation.
func guildOpsStatus(c *gin.Context, services *quack.Services) {
	guildID, ok := guildOpsAuthorized(c, services)
	if !ok {
		return
	}

	status, err := services.Ops.GuildStatus(c.Request.Context(), guildID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "ops status failed"})
		return
	}
	c.JSON(http.StatusOK, status)
}

// guildOpsAuthorized encapsulates the guild ops authorized rule so callers share one consistent package implementation.
func guildOpsAuthorized(c *gin.Context, services *quack.Services) (string, bool) {
	discordGuildID := strings.TrimSpace(c.Param("discordGuildID"))
	if discordGuildID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing discord guild id"})
		return "", false
	}

	if validOpsKey(c, services) {
		guild, err := services.Store.GetGuildByDiscordID(c.Request.Context(), discordGuildID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "guild lookup failed"})
			return "", false
		}
		if guild == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "guild not found"})
			return "", false
		}
		return guild.ID, true
	}

	sessionID := middleware.ExtractSessionID(c, services.Config.Auth.SessionCookieName)
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing auth session"})
		return "", false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	session, err := services.Store.GetSession(ctx, sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load auth session"})
		return "", false
	}
	if session == nil || session.DiscordUserID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid auth session"})
		return "", false
	}
	if !session.SessionExpiresAt.IsZero() && time.Now().UTC().After(session.SessionExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "auth session expired"})
		return "", false
	}

	guildContext, err := services.Guilds.ResolveStaffContext(ctx, session, discordGuildID)
	if err != nil {
		status := http.StatusForbidden
		if errors.Is(err, quack.ErrBotNotInGuild) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return "", false
	}
	if !guildContext.IsAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "guild administrator access required"})
		return "", false
	}
	return guildContext.Guild.ID, true
}

// validOpsKey checks valid ops key before state is read or changed.
func validOpsKey(c *gin.Context, services *quack.Services) bool {
	if services == nil {
		return false
	}
	configured := strings.TrimSpace(services.Config.API.OpsStatusToken)
	if configured == "" {
		return false
	}
	return c.GetHeader(opsKeyHeader) == configured
}
