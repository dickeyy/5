package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/discord"
)

func status(c *gin.Context) {
	discordStatus := "disconnected"
	if discord.Session.State.User != nil {
		discordStatus = "connected"
	}

	user := discord.Session.State.User
	var userData any = nil
	if user != nil {
		discordStatus = "connected"
		userData = gin.H{
			"id":         user.ID,
			"username":   user.Username,
			"avatar_url": user.AvatarURL(""),
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"discord": gin.H{
			"status": discordStatus,
			"user":   userData,
		},
	})
}
