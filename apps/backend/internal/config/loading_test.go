package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestParseEnvironmentDefaults guards the shared defaults used by startup and tests.
func TestParseEnvironmentDefaults(t *testing.T) {
	for _, values := range []map[string]string{{}, {"EVENT_QUEUE_WORKERS": "", "AUTH_COOKIE_SECURE": ""}} {
		cfg, err := parseEnvironment(values)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(cfg, Default()) {
			t.Fatalf("loaded defaults differ from Default: %+v", cfg)
		}
	}
}

// TestParseEnvironmentModes keeps development credentials and defaults isolated.
func TestParseEnvironmentModes(t *testing.T) {
	for _, mode := range []string{"dev", "test", "staging", "production"} {
		t.Run(mode, func(t *testing.T) {
			cfg, err := parseEnvironment(map[string]string{
				"ENVIRONMENT":   mode,
				"DISCORD_TOKEN": "normal-token", "DEV_DISCORD_TOKEN": "dev-token",
				"DISCORD_APP_ID": "normal-app", "DEV_DISCORD_APP_ID": "dev-app",
				"DISCORD_CLIENT_SECRET": "normal-secret", "DEV_DISCORD_CLIENT_SECRET": "dev-secret",
			})
			if err != nil {
				t.Fatal(err)
			}
			prefix := "normal-"
			if mode == "dev" {
				prefix = "dev-"
			}
			if cfg.Discord.Token != prefix+"token" || cfg.Discord.AppID != prefix+"app" || cfg.Discord.ClientSecret != prefix+"secret" {
				t.Fatalf("wrong credentials for %s", mode)
			}
			if cfg.Auth.CookieSecure != (mode != "dev") {
				t.Fatal("wrong secure cookie default")
			}
			if (len(cfg.API.CORSAllowedOrigins) > 0) != (mode == "dev") {
				t.Fatal("wrong CORS defaults")
			}
		})
	}
	cfg, err := parseEnvironment(map[string]string{"DISCORD_TOKEN": "production-token", "DISCORD_APP_ID": "production-app"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Discord.Token != "" || cfg.Discord.AppID != "" {
		t.Fatal("development must not fall back to production credentials")
	}
}

// TestParseEnvironmentOverrides verifies typed settings, nested policies, and list normalization.
func TestParseEnvironmentOverrides(t *testing.T) {
	cfg, err := parseEnvironment(map[string]string{
		"ENVIRONMENT": "production", "API_PORT": "9090", "API_MAX_BODY_BYTES": "4294967296",
		"AUTH_COOKIE_SECURE": "false", "DISCORD_COMMAND_PRUNE": "true",
		"EVENT_QUEUE_WORKERS": "7", "RATE_LIMIT_OAUTH_MAXIMUM": "42",
		"RATE_LIMIT_EVIDENCE_WINDOW_SECONDS": "123", "HTTP_IDEMPOTENCY_TTL_HOURS": "12",
		"API_CORS_ALLOWED_ORIGINS": " https://example.com, ,https://other.example.com ,",
		"API_TRUSTED_PROXIES":      " 127.0.0.1, ,10.0.0.0/8 ", "LOG_LEVEL": "debug",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.API.Port != "9090" || cfg.API.MaxBodyBytes != 4294967296 || cfg.Auth.CookieSecure || !cfg.Discord.CommandPrune || cfg.EventQueue.Workers != 7 || cfg.Observability.LogLevel != "debug" {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
	if cfg.RateLimits.OAuth.Maximum != 42 || cfg.RateLimits.OAuth.WindowSeconds != 600 || cfg.RateLimits.Evidence.WindowSeconds != 123 || cfg.RateLimits.IdempotencyTTLHours != 12 {
		t.Fatalf("wrong rate limits: %+v", cfg.RateLimits)
	}
	if !reflect.DeepEqual(cfg.API.CORSAllowedOrigins, []string{"https://example.com", "https://other.example.com"}) || !reflect.DeepEqual(cfg.API.TrustedProxies, []string{"127.0.0.1", "10.0.0.0/8"}) {
		t.Fatalf("wrong lists: %+v", cfg.API)
	}
}

// TestParseEnvironmentRejectsMalformedValues prevents silent fallback on decoding failures.
func TestParseEnvironmentRejectsMalformedValues(t *testing.T) {
	for _, key := range []string{"API_MAX_BODY_BYTES", "EVENT_QUEUE_WORKERS", "AUTH_COOKIE_SECURE", "DISCORD_COMMAND_PRUNE", "RATE_LIMIT_RETRY_MAXIMUM"} {
		t.Run(key, func(t *testing.T) {
			if _, err := parseEnvironment(map[string]string{key: "invalid"}); err == nil {
				t.Fatal("expected parsing error")
			}
		})
	}
	if _, err := parseEnvironment(map[string]string{"API_MAX_BODY_BYTES": "9223372036854775808"}); err == nil {
		t.Fatal("expected overflow error")
	}
}

// TestLoadDotEnvPrecedence exercises file merging without mutating the process environment.
func TestLoadDotEnvPrecedence(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ENVIRONMENT", "dev")
	t.Setenv("API_PORT", "9090")
	// Restore the caller's value while leaving this setting absent for the file test.
	t.Setenv("DEV_DISCORD_TOKEN", "")
	if err := os.Unsetenv("DEV_DISCORD_TOKEN"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(".env", []byte("API_PORT=9999\nDEV_DISCORD_TOKEN=file-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.API.Port != "9090" || cfg.Discord.Token != "file-token" {
		t.Fatal("wrong .env precedence")
	}
	if _, exists := os.LookupEnv("DEV_DISCORD_TOKEN"); exists {
		t.Fatal("Load mutated the process environment")
	}
	t.Setenv("ENVIRONMENT", "production")
	if err := os.WriteFile(".env", []byte("EVENT_QUEUE_WORKERS=invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err != nil {
		t.Fatalf("production read .env: %v", err)
	}
}

// TestValidateUsesLoadedValues ensures validation is independent of later environment changes.
func TestValidateUsesLoadedValues(t *testing.T) {
	cfg := validStartupConfig()
	t.Setenv("EVENT_QUEUE_WORKERS", "invalid")
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validation read process environment: %v", err)
	}
	cfg.API.MaxBodyBytes = 0
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "API_MAX_BODY_BYTES") {
		t.Fatalf("expected body size error, got %v", err)
	}
	cfg.API.MaxBodyBytes = Default().API.MaxBodyBytes
	cfg.RateLimits.Evidence.WindowSeconds = -1
	if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), "RATE_LIMIT_EVIDENCE") {
		t.Fatalf("expected rate limit error, got %v", err)
	}
}
