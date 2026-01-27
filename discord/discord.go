package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/lib"
	"github.com/quackdiscord/bot/structs"
	"github.com/rs/zerolog/log"
)

// TODO: add a command hash cache to efficiently re-register commands when needed

var Session *discordgo.Session
var Commands map[string]*structs.DiscordCommand

// Connect inits the session and opens the connection
func Connect() {
	log.Info().Msg("Connecting to Discord")

	token := lib.Config.Discord.Token

	// create the session
	var err error
	Session, err = discordgo.New(token)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create discord session")
	}

	// apply intents and state
	Session.Identify.Intents = discordgo.Intent(3276543)
	Session.StateEnabled = true
	Session.State.MaxMessageCount = 5000

	// open the session
	err = Session.Open()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open discord session")
	}

	// register commands here later
}
