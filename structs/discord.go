package structs

import "github.com/bwmarrin/discordgo"

type DiscordCommand struct {
	*discordgo.ApplicationCommand
	Handler func(s *discordgo.Session, i *discordgo.InteractionCreate) *discordgo.InteractionResponse
}
