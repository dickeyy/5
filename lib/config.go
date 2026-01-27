package lib

import (
	"os"

	"github.com/quackdiscord/bot/structs"
)

var Config *structs.Config

func init() {
	Config = &structs.Config{
		API: structs.APIConfig{
			Port: getEnvWithDefault("API_PORT", "8080"),
		},
	}
}

func getEnvWithDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
