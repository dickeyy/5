package events

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/discord"
	"github.com/quackdiscord/bot/services"
	"github.com/quackdiscord/bot/structs"
)

func init() {
	discord.Events["message_delete"] = &structs.DiscordEvent{
		Handler: msgDelete,
	}
}

func msgDelete(_ *discordgo.Session, data *discordgo.MessageDelete) {
	services.EQ.Enqueue(structs.QueueEvent{
		Type:    "message_delete",
		Data:    data,
		Handler: msgDeleteHandler,
	})
}

func msgDeleteHandler(ctx context.Context, s structs.DataStore, data any) error {
	// msg := data.(*discordgo.MessageDelete)
	// do something here
	_ = ctx
	_ = s
	return nil
}
