package lib

import (
	"fmt"
	"os"

	"github.com/quackdiscord/bot/structs"
)

var Config *structs.Config

func LoadConfig() {
	env := getEnvWithDefault("ENVIRONMENT", "dev")
	Config = &structs.Config{
		Environment: env,
		API: structs.APIConfig{
			Port: getEnvWithDefault("API_PORT", "8080"),
		},
		Discord: structs.DiscordConfig{
			Token: getEnvWithEnvironmentOverride("DISCORD_TOKEN", env),
			AppID: getEnvWithEnvironmentOverride("DISCORD_APP_ID", env),
		},
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvWithEnvironmentOverride(key, env string) string {
	if env == "dev" {
		if value, exists := os.LookupEnv(fmt.Sprintf("DEV_%s", key)); exists {
			return value
		}
	} else {
		if value, exists := os.LookupEnv(key); exists {
			return value
		}
	}
	return ""
}
