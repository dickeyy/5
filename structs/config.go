package structs

type Config struct {
	Environment string
	API         APIConfig
	Discord     DiscordConfig
	Storage     StorageConfig
}

type APIConfig struct {
	Port string
}

type DiscordConfig struct {
	Token string
	AppID string
}

type StorageConfig struct {
	DBDSN    string
	RedisURL string
}
