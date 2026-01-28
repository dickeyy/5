package structs

type Config struct {
	Environment string
	API         APIConfig
	Discord     DiscordConfig
	Storage     StorageConfig
	EventQueue  EventQueueConfig
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

type EventQueueConfig struct {
	Size    int
	Workers int
}
