package main

import (
	"os"

	"github.com/quackdiscord/bot/api"
	"github.com/quackdiscord/bot/discord"
	"github.com/quackdiscord/bot/lib"
	"github.com/quackdiscord/bot/services"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	log.Logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).With().Timestamp().Caller().Logger()
	lib.LoadConfig()
}

func main() {
	services.DB.Connect()
	services.Redis.Connect()

	services.EQ.Init()
	services.EQ.Start()

	discord.Connect()
	api.Start()
}
