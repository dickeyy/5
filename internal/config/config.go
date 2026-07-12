package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

// Load reads environment configuration once and validates values needed to construct the process.
func Load() Config {
	if err := godotenv.Load(".env"); err != nil {
		log.Warn().Msg("Error loading .env file")
	}

	env := getEnvWithDefault("ENVIRONMENT", "dev")
	corsAllowedOrigins := getEnvCSV("API_CORS_ALLOWED_ORIGINS")
	if env == "dev" && len(corsAllowedOrigins) == 0 {
		corsAllowedOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	}
	return Config{
		Environment: env,
		API: APIConfig{
			Port:                     getEnvWithDefault("API_PORT", "8080"),
			OpsStatusToken:           getEnvWithDefault("OPS_STATUS_TOKEN", ""),
			CORSAllowedOrigins:       corsAllowedOrigins,
			MaxBodyBytes:             getEnvWithDefaultInt64("API_MAX_BODY_BYTES", 1<<20),
			ReadHeaderTimeoutSeconds: getEnvWithDefaultInt("API_READ_HEADER_TIMEOUT_SECONDS", 5),
			ReadTimeoutSeconds:       getEnvWithDefaultInt("API_READ_TIMEOUT_SECONDS", 15),
			WriteTimeoutSeconds:      getEnvWithDefaultInt("API_WRITE_TIMEOUT_SECONDS", 30),
			IdleTimeoutSeconds:       getEnvWithDefaultInt("API_IDLE_TIMEOUT_SECONDS", 60),
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
			CSRFCookieName:    getEnvWithDefault("AUTH_CSRF_COOKIE_NAME", "quack_csrf"),
			SessionTTLHours:   getEnvWithDefaultInt("AUTH_SESSION_TTL_HOURS", 168),
			StateTTLMinutes:   getEnvWithDefaultInt("AUTH_STATE_TTL_MINUTES", 10),
			PostLoginRedirect: getEnvWithDefault("AUTH_POST_LOGIN_REDIRECT", "/"),
			CookieSecure:      getEnvWithDefaultBool("AUTH_COOKIE_SECURE", env != "dev"),
		},
		RateLimits: RateLimitConfig{
			OAuth:               rateLimitPolicyFromEnv("OAUTH", 20, 600),
			MemberRead:          rateLimitPolicyFromEnv("MEMBER_READ", 120, 60),
			TemplateWrite:       rateLimitPolicyFromEnv("TEMPLATE_WRITE", 30, 60),
			CaseCreate:          rateLimitPolicyFromEnv("CASE_CREATE", 20, 60),
			Retry:               rateLimitPolicyFromEnv("RETRY", 10, 60),
			Evidence:            rateLimitPolicyFromEnv("EVIDENCE", 20, 60),
			IdempotencyTTLHours: getEnvWithDefaultInt("HTTP_IDEMPOTENCY_TTL_HOURS", 24),
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
		API: APIConfig{
			Port:                     "8080",
			CORSAllowedOrigins:       []string{"http://localhost:3000", "http://127.0.0.1:3000"},
			MaxBodyBytes:             1 << 20,
			ReadHeaderTimeoutSeconds: 5,
			ReadTimeoutSeconds:       15,
			WriteTimeoutSeconds:      30,
			IdleTimeoutSeconds:       60,
		},
		Auth: AuthConfig{
			SessionCookieName: "quack_session",
			CSRFCookieName:    "quack_csrf",
			SessionTTLHours:   168,
			StateTTLMinutes:   10,
			PostLoginRedirect: "/",
		},
		RateLimits: defaultRateLimits(),
		EventQueue: EventQueueConfig{Size: 1000, Workers: 3},
	}
}

// defaultRateLimits returns the product-wide adapter policy defaults used in tests and development.
func defaultRateLimits() RateLimitConfig {
	return RateLimitConfig{
		OAuth:               RateLimitPolicyConfig{Maximum: 20, WindowSeconds: 600},
		MemberRead:          RateLimitPolicyConfig{Maximum: 120, WindowSeconds: 60},
		TemplateWrite:       RateLimitPolicyConfig{Maximum: 30, WindowSeconds: 60},
		CaseCreate:          RateLimitPolicyConfig{Maximum: 20, WindowSeconds: 60},
		Retry:               RateLimitPolicyConfig{Maximum: 10, WindowSeconds: 60},
		Evidence:            RateLimitPolicyConfig{Maximum: 20, WindowSeconds: 60},
		IdempotencyTTLHours: 24,
	}
}

// rateLimitPolicyFromEnv reads one documented endpoint-class policy.
func rateLimitPolicyFromEnv(class string, maximum, windowSeconds int) RateLimitPolicyConfig {
	return RateLimitPolicyConfig{
		Maximum:       getEnvWithDefaultInt("RATE_LIMIT_"+class+"_MAXIMUM", maximum),
		WindowSeconds: getEnvWithDefaultInt("RATE_LIMIT_"+class+"_WINDOW_SECONDS", windowSeconds),
	}
}

// getEnvCSV returns trimmed, non-empty comma-separated environment values.
func getEnvCSV(key string) []string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
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

// getEnvWithDefaultInt64 retrieves a positive-width integer environment value.
func getEnvWithDefaultInt64(key string, defaultValue int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		intValue, err := strconv.ParseInt(value, 10, 64)
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
