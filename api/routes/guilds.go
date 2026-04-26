package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/app"
)

func guildMe(c *gin.Context) {
	guildContext := middleware.GetGuildContext(c)
	if guildContext == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "missing guild context"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"guild": gin.H{
			"id":                    guildContext.Guild.ID,
			"discord_guild_id":      guildContext.Guild.DiscordGuildID,
			"name":                  guildContext.Guild.Name,
			"icon_url":              guildContext.Guild.IconURL,
			"owner_discord_user_id": guildContext.Guild.OwnerDiscordUserID,
			"rollout_state":         guildContext.Guild.RolloutState,
		},
		"staff": gin.H{
			"id":                    guildContext.Staff.ID,
			"discord_user_id":       guildContext.Staff.DiscordUserID,
			"display_name":          guildContext.Staff.LastKnownDisplayName,
			"permission_bits":       app.PermissionBitsString(guildContext.PermissionBits),
			"is_owner":              guildContext.IsOwner,
			"is_administrator":      guildContext.IsAdministrator,
			"disabled":              guildContext.Staff.DisabledAt != nil,
			"last_active_at":        guildContext.Staff.LastActiveAt,
			"last_seen_permissions": app.PermissionBitsString(guildContext.Staff.LastSeenPermissionBits),
		},
		"permissions": app.PermissionMapStrings(guildContext.Permissions),
	})
}
