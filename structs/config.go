package structs

type Config struct {
	Environment string
	API         APIConfig
	Discord     DiscordConfig
}

type APIConfig struct {
	Port string
}

type DiscordConfig struct {
	Token string
	AppID string
}
