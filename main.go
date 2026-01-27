package main

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/quackdiscord/bot/api"
	"github.com/quackdiscord/bot/discord"
	"github.com/quackdiscord/bot/lib"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Caller().Logger()

	if err := godotenv.Load(".env"); err != nil {
		log.Warn().Msg("Error loading .env file")
	}

	lib.LoadConfig()
}

func main() {
	discord.Connect()
	api.Start()
}
