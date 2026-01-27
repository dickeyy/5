package routes

import (
	"net/http"

	"github.com/quackdiscord/bot/api/respond"
	"github.com/quackdiscord/bot/discord"
)

type statusResponse struct {
	Discord statusResponseDiscord `json:"discord"`
}

type statusResponseDiscord struct {
	Status string                    `json:"status"`
	User   statusResponseDiscordUser `json:"user"`
}

type statusResponseDiscordUser struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func status(w http.ResponseWriter, r *http.Request) {
	discordStatus := "disconnected"
	if discord.Session.State.User != nil {
		discordStatus = "connected"
	}

	respond.JSON(w, http.StatusOK, statusResponse{
		Discord: statusResponseDiscord{
			Status: discordStatus,
			User: statusResponseDiscordUser{
				ID:        discord.Session.State.User.ID,
				Username:  discord.Session.State.User.Username,
				AvatarURL: discord.Session.State.User.AvatarURL(""),
			},
		},
	})
}
