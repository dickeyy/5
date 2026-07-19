package config

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

// Load reads environment configuration once and validates values needed to construct the process.
func Load() Config {
	env := getEnvWithDefault("ENVIRONMENT", "dev")
	if env == "dev" {
		if err := godotenv.Load(".env"); err != nil {
			log.Debug().Msg("Development .env file was not loaded")
		}
		env = getEnvWithDefault("ENVIRONMENT", "dev")
	}
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
			TrustedProxies:           getEnvCSV("API_TRUSTED_PROXIES"),
			MaxBodyBytes:             getEnvWithDefaultInt64("API_MAX_BODY_BYTES", 1<<20),
			ReadHeaderTimeoutSeconds: getEnvWithDefaultInt("API_READ_HEADER_TIMEOUT_SECONDS", 5),
			ReadTimeoutSeconds:       getEnvWithDefaultInt("API_READ_TIMEOUT_SECONDS", 15),
			WriteTimeoutSeconds:      getEnvWithDefaultInt("API_WRITE_TIMEOUT_SECONDS", 30),
			IdleTimeoutSeconds:       getEnvWithDefaultInt("API_IDLE_TIMEOUT_SECONDS", 60),
			ShutdownTimeoutSeconds:   getEnvWithDefaultInt("SHUTDOWN_TIMEOUT_SECONDS", 20),
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
		Observability: ObservabilityConfig{
			MetricsToken: getEnvWithDefault("METRICS_TOKEN", ""),
			ServiceName:  getEnvWithDefault("SERVICE_NAME", "quack"),
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
			ShutdownTimeoutSeconds:   20,
		},
		Auth: AuthConfig{
			SessionCookieName: "quack_session",
			CSRFCookieName:    "quack_csrf",
			SessionTTLHours:   168,
			StateTTLMinutes:   10,
			PostLoginRedirect: "/",
		},
		Discord:       DiscordConfig{OAuthScopes: "identify guilds"},
		RateLimits:    defaultRateLimits(),
		EventQueue:    EventQueueConfig{Size: 1000, Workers: 3},
		Observability: ObservabilityConfig{ServiceName: "quack"},
	}
}

// Validate rejects incomplete or unsafe startup configuration before any
// database, Redis, Discord, worker, or listener side effect occurs.
func (c Config) Validate() error {
	if err := validateRawEnvironment(); err != nil {
		return err
	}
	if c.Environment != "dev" && c.Environment != "test" && c.Environment != "staging" && c.Environment != "production" {
		return fmt.Errorf("ENVIRONMENT must be one of dev, test, staging, or production")
	}
	if strings.TrimSpace(c.API.Port) == "" {
		return fmt.Errorf("API_PORT is required")
	}
	if c.API.ShutdownTimeoutSeconds <= 0 {
		return fmt.Errorf("SHUTDOWN_TIMEOUT_SECONDS must be positive")
	}
	if c.EventQueue.Size <= 0 || c.EventQueue.Workers <= 0 {
		return fmt.Errorf("EVENT_QUEUE_SIZE and EVENT_QUEUE_WORKERS must be positive")
	}
	if strings.TrimSpace(c.Observability.ServiceName) == "" {
		return fmt.Errorf("SERVICE_NAME is required")
	}
	if strings.TrimSpace(c.Storage.DBDSN) == "" || strings.TrimSpace(c.Storage.RedisURL) == "" {
		return fmt.Errorf("DATABASE_DSN and REDIS_URL are required")
	}
	if strings.TrimSpace(c.Discord.Token) == "" || strings.TrimSpace(c.Discord.AppID) == "" {
		return fmt.Errorf("DISCORD_TOKEN and DISCORD_APP_ID are required")
	}
	if c.Environment == "staging" || c.Environment == "production" {
		if strings.TrimSpace(c.Discord.ClientSecret) == "" || strings.TrimSpace(c.Discord.OAuthRedirectURI) == "" {
			return fmt.Errorf("DISCORD_CLIENT_SECRET and DISCORD_OAUTH_REDIRECT_URI are required outside development")
		}
		if strings.TrimSpace(c.API.OpsStatusToken) == "" || strings.TrimSpace(c.Observability.MetricsToken) == "" {
			return fmt.Errorf("OPS_STATUS_TOKEN and METRICS_TOKEN are required outside development")
		}
		redirect, err := url.Parse(c.Discord.OAuthRedirectURI)
		if err != nil || redirect.Scheme != "https" || redirect.Host == "" || redirect.User != nil || redirect.RawQuery != "" || redirect.Fragment != "" {
			return fmt.Errorf("DISCORD_OAUTH_REDIRECT_URI must be an exact HTTPS URL outside development")
		}
		if !slices.Contains(strings.Fields(c.Discord.OAuthScopes), "identify") || !slices.Contains(strings.Fields(c.Discord.OAuthScopes), "guilds") {
			return fmt.Errorf("DISCORD_OAUTH_SCOPES must include identify and guilds")
		}
	}
	return nil
}

// validateRawEnvironment prevents malformed values from silently falling back
// to defaults during production startup.
func validateRawEnvironment() error {
	integers := []string{
		"API_MAX_BODY_BYTES", "API_READ_HEADER_TIMEOUT_SECONDS", "API_READ_TIMEOUT_SECONDS",
		"API_WRITE_TIMEOUT_SECONDS", "API_IDLE_TIMEOUT_SECONDS", "SHUTDOWN_TIMEOUT_SECONDS",
		"AUTH_SESSION_TTL_HOURS", "AUTH_STATE_TTL_MINUTES", "EVENT_QUEUE_SIZE", "EVENT_QUEUE_WORKERS",
		"RATE_LIMIT_OAUTH_MAXIMUM", "RATE_LIMIT_OAUTH_WINDOW_SECONDS", "RATE_LIMIT_MEMBER_READ_MAXIMUM",
		"RATE_LIMIT_MEMBER_READ_WINDOW_SECONDS", "RATE_LIMIT_TEMPLATE_WRITE_MAXIMUM", "RATE_LIMIT_TEMPLATE_WRITE_WINDOW_SECONDS",
		"RATE_LIMIT_CASE_CREATE_MAXIMUM", "RATE_LIMIT_CASE_CREATE_WINDOW_SECONDS", "RATE_LIMIT_RETRY_MAXIMUM",
		"RATE_LIMIT_RETRY_WINDOW_SECONDS", "RATE_LIMIT_EVIDENCE_MAXIMUM", "RATE_LIMIT_EVIDENCE_WINDOW_SECONDS",
		"HTTP_IDEMPOTENCY_TTL_HOURS",
	}
	for _, key := range integers {
		if raw, ok := os.LookupEnv(key); ok {
			value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil || value <= 0 {
				return fmt.Errorf("%s must be a positive integer", key)
			}
		}
	}
	for _, key := range []string{"AUTH_COOKIE_SECURE", "DISCORD_COMMAND_PRUNE"} {
		if raw, ok := os.LookupEnv(key); ok {
			if _, err := strconv.ParseBool(strings.TrimSpace(raw)); err != nil {
				return fmt.Errorf("%s must be true or false", key)
			}
		}
	}
	return nil
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

// getCriticalEnv returns a required value for later aggregate validation. It
// deliberately does not terminate the process from a parsing helper.
func getCriticalEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return ""
}
