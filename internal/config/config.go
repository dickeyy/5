package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

// Load reads environment configuration once and validates values needed to construct the process.
func Load() Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Warn().Msg("Error loading .env file")
	}

	env := getEnvWithDefault("ENVIRONMENT", "dev")
	return Config{
		Environment: env,
		API: APIConfig{
			Port:           getEnvWithDefault("API_PORT", "8080"),
			OpsStatusToken: getEnvWithDefault("OPS_STATUS_TOKEN", ""),
		},
		Discord: DiscordConfig{
			Token:            getEnvWithEnvironmentOverride("DISCORD_TOKEN", env, true),
			AppID:            getEnvWithEnvironmentOverride("DISCORD_APP_ID", env, true),
			ClientSecret:     getEnvWithEnvironmentOverride("DISCORD_CLIENT_SECRET", env, false),
			OAuthRedirectURI: getEnvWithDefault("DISCORD_OAUTH_REDIRECT_URI", ""),
			OAuthScopes:      getEnvWithDefault("DISCORD_OAUTH_SCOPES", "identify guilds"),
			CommandGuildID:   getEnvWithDefault("DISCORD_COMMAND_GUILD_ID", ""),
			CommandPrune:     getEnvWithDefaultBool("DISCORD_COMMAND_PRUNE", false),
		},
		Auth: AuthConfig{
			SessionCookieName: getEnvWithDefault("AUTH_SESSION_COOKIE_NAME", "quack_session"),
			SessionTTLHours:   getEnvWithDefaultInt("AUTH_SESSION_TTL_HOURS", 168),
			StateTTLMinutes:   getEnvWithDefaultInt("AUTH_STATE_TTL_MINUTES", 10),
			PostLoginRedirect: getEnvWithDefault("AUTH_POST_LOGIN_REDIRECT", "/"),
			CookieSecure:      getEnvWithDefaultBool("AUTH_COOKIE_SECURE", env != "dev"),
		},
		Storage: StorageConfig{
			DBDSN:    getCriticalEnv("DATABASE_DSN"),
			RedisURL: getCriticalEnv("REDIS_URL"),
		},
		EventQueue: EventQueueConfig{
			Size:    getEnvWithDefaultInt("EVENT_QUEUE_SIZE", 1000),
			Workers: getEnvWithDefaultInt("EVENT_QUEUE_WORKERS", 3),
		},
	}
}

// Default returns development-safe configuration defaults before environment overrides are applied.
func Default() Config {
	return Config{
		Environment: "dev",
		API:         APIConfig{Port: "8080"},
		Auth: AuthConfig{
			SessionCookieName: "quack_session",
			SessionTTLHours:   168,
			StateTTLMinutes:   10,
			PostLoginRedirect: "/",
		},
		EventQueue: EventQueueConfig{Size: 1000, Workers: 3},
	}
}

// getEnvWithDefault returns the value of an environment variable if
// it exists, otherwise it returns the default value
func getEnvWithDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvWithDefaultInt retrieves env with default int without exposing the underlying adapter implementation.
func getEnvWithDefaultInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			log.Warn().Str("key", key).Msg("Invalid environment variable")
			return defaultValue
		}
		return intValue
	}
	return defaultValue
}

// getEnvWithDefaultBool retrieves env with default bool without exposing the underlying adapter implementation.
func getEnvWithDefaultBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		boolValue, err := strconv.ParseBool(value)
		if err != nil {
			log.Warn().Str("key", key).Msg("Invalid boolean environment variable")
			return defaultValue
		}
		return boolValue
	}
	return defaultValue
}

// getEnvWithEnvironmentOverride returns the value of an environmet variable
// if the env is dev, it will append DEV_ to the key
func getEnvWithEnvironmentOverride(key, env string, critical bool) string {
	if env == "dev" {
		if value, exists := os.LookupEnv(fmt.Sprintf("DEV_%s", key)); exists {
			return value
		}
	} else {
		if value, exists := os.LookupEnv(key); exists {
			return value
		}
	}

	if critical {
		getCriticalEnv(key)
	}
	return ""
}

// getCriticalEnv returns the value of an env variable,
// if it does not exist, the program will panic
func getCriticalEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	log.Fatal().Str("key", key).Msg("Critical environment variable not set")
	return ""
}
