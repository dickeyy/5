package lib

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/quackdiscord/bot/structs"
	"github.com/rs/zerolog/log"
)

var Config *structs.Config

func LoadConfig() {
	if err := godotenv.Load(".env"); err != nil {
		log.Warn().Msg("Error loading .env file")
	}

	env := getEnvWithDefault("ENVIRONMENT", "dev")
	Config = &structs.Config{
		Environment: env,
		API: structs.APIConfig{
			Port: getEnvWithDefault("API_PORT", "8080"),
		},
		Discord: structs.DiscordConfig{
			Token: getEnvWithEnvironmentOverride("DISCORD_TOKEN", env, true),
			AppID: getEnvWithEnvironmentOverride("DISCORD_APP_ID", env, true),
		},
		Storage: structs.StorageConfig{
			DBDSN:    getCriticalEnv("DATABASE_DSN"),
			RedisURL: getCriticalEnv("REDIS_URL"),
		},
		EventQueue: structs.EventQueueConfig{
			Size:    getEnvWithDefaultInt("EVENT_QUEUE_SIZE", 1000),
			Workers: getEnvWithDefaultInt("EVENT_QUEUE_WORKERS", 3),
		},
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
