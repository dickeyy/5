package config

// Config is the complete immutable process configuration passed into runtime assembly.
type Config struct {
	Environment string
	API         APIConfig
	Discord     DiscordConfig
	Auth        AuthConfig
	RateLimits  RateLimitConfig
	Storage     StorageConfig
	EventQueue  EventQueueConfig
}

// RateLimitConfig defines documented fail-closed limits for dashboard and Discord adapter classes.
type RateLimitConfig struct {
	OAuth               RateLimitPolicyConfig
	MemberRead          RateLimitPolicyConfig
	TemplateWrite       RateLimitPolicyConfig
	CaseCreate          RateLimitPolicyConfig
	Retry               RateLimitPolicyConfig
	Evidence            RateLimitPolicyConfig
	IdempotencyTTLHours int
}

// RateLimitPolicyConfig defines a maximum request count within a fixed window.
type RateLimitPolicyConfig struct {
	Maximum       int
	WindowSeconds int
}

// APIConfig controls the HTTP listener and privileged operations endpoint.
type APIConfig struct {
	Port                     string
	OpsStatusToken           string
	CORSAllowedOrigins       []string
	TrustedProxies           []string
	MaxBodyBytes             int64
	ReadHeaderTimeoutSeconds int
	ReadTimeoutSeconds       int
	WriteTimeoutSeconds      int
	IdleTimeoutSeconds       int
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
	CSRFCookieName    string
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
