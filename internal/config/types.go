package config

// Config is the complete immutable process configuration passed into runtime assembly.
type Config struct {
	Environment string
	API         APIConfig
	Discord     DiscordConfig
	Auth        AuthConfig
	Storage     StorageConfig
	EventQueue  EventQueueConfig
}

// APIConfig controls the HTTP listener and privileged operations endpoint.
type APIConfig struct {
	Port           string
	OpsStatusToken string
}

// DiscordConfig contains Discord credentials, OAuth settings, and command synchronization policy.
type DiscordConfig struct {
	Token            string
	AppID            string
	ClientSecret     string
	OAuthRedirectURI string
	OAuthScopes      string
	CommandGuildID   string
	CommandPrune     bool
}

// AuthConfig controls session lifetime, cookie behavior, and the post-login destination.
type AuthConfig struct {
	SessionCookieName string
	SessionTTLHours   int
	StateTTLMinutes   int
	PostLoginRedirect string
	CookieSecure      bool
}

// StorageConfig identifies the MySQL and Redis instances used by adapters.
type StorageConfig struct {
	DBDSN    string
	RedisURL string
}

// EventQueueConfig bounds the in-process action queue and worker concurrency.
type EventQueueConfig struct {
	Size    int
	Workers int
}
