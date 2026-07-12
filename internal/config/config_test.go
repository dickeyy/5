package config

import (
	"strings"
	"testing"
)

func validStartupConfig() Config {
	cfg := Default()
	cfg.Storage = StorageConfig{DBDSN: "user:password@tcp(database:3306)/quack", RedisURL: "redis://redis:6379/0"}
	cfg.Discord = DiscordConfig{Token: "bot-token", AppID: "application-id", OAuthScopes: "identify guilds"}
	return cfg
}

func TestValidateRejectsIncompleteAndUnsafeStartupConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{name: "storage", mutate: func(cfg *Config) { cfg.Storage.DBDSN = "" }, want: "DATABASE_DSN"},
		{name: "discord", mutate: func(cfg *Config) { cfg.Discord.Token = "" }, want: "DISCORD_TOKEN"},
		{name: "queue", mutate: func(cfg *Config) { cfg.EventQueue.Workers = 0 }, want: "EVENT_QUEUE"},
		{name: "shutdown", mutate: func(cfg *Config) { cfg.API.ShutdownTimeoutSeconds = 0 }, want: "SHUTDOWN_TIMEOUT_SECONDS"},
		{name: "service", mutate: func(cfg *Config) { cfg.Observability.ServiceName = "" }, want: "SERVICE_NAME"},
		{name: "environment", mutate: func(cfg *Config) { cfg.Environment = "prod-ish" }, want: "ENVIRONMENT"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validStartupConfig()
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected actionable %s error, got %v", test.want, err)
			}
		})
	}
}

func TestValidateRequiresProductionOAuthAndOperatorSecrets(t *testing.T) {
	cfg := validStartupConfig()
	cfg.Environment = "production"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected incomplete production configuration to fail")
	}
	cfg.Discord.ClientSecret = "client-secret"
	cfg.Discord.OAuthRedirectURI = "https://dashboard.example.com/auth/callback"
	cfg.API.OpsStatusToken = "ops-secret"
	cfg.Observability.MetricsToken = "metrics-secret"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected complete production configuration: %v", err)
	}
}
