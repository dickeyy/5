package events

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/discord"
	"github.com/quackdiscord/bot/services"
	"github.com/quackdiscord/bot/structs"
)

func init() {
	discord.Events["message_create"] = &structs.DiscordEvent{
		Handler: msgCreate,
	}
}

// msgCreate is the Discord event handler that enqueues the event
func msgCreate(_ *discordgo.Session, data *discordgo.MessageCreate) {
	services.EQ.Enqueue(structs.QueueEvent{
		Type:    "message_create",
		Data:    data,
		Handler: msgCreateHandler,
	})
}

// msgCreateHandler is the actual handler that processes the message create event
func msgCreateHandler(ctx context.Context, s structs.DataStore, data any) error {
	// msg := data.(*discordgo.MessageCreate)
	// do something here
	_ = ctx
	_ = s
	return nil
}
