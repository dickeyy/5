package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/quack"
)

// DiscordStatusProvider supplies discord status data without coupling callers to its source.
type DiscordStatusProvider interface {
	Status() (connected bool, username string, latencyMS int64)
}

// discordStatus identifies the supported discord status values stored and exchanged by Quack.
type discordStatus struct {
	Connected bool   `json:"connected"`
	Username  string `json:"username,omitempty"`
	Latency   int64  `json:"latency,omitempty"`
}

// redisStatus identifies the supported redis status values stored and exchanged by Quack.
type redisStatus struct {
	Connected bool  `json:"connected"`
	Latency   int64 `json:"latency,omitempty"`
}

// dbStatus identifies the supported db status values stored and exchanged by Quack.
type dbStatus struct {
	Connected bool  `json:"connected"`
	Latency   int64 `json:"latency,omitempty"`
}

// status encapsulates the status rule so callers share one consistent package implementation.
func status(c *gin.Context, services *quack.Services, discord DiscordStatusProvider) {
	c.JSON(http.StatusOK, gin.H{
		"discord":  getDiscordStatus(discord),
		"redis":    getRedisStatus(services),
		"database": getDBStatus(services),
	})
}

// getDiscordStatus retrieves discord status without exposing the underlying adapter implementation.
func getDiscordStatus(provider DiscordStatusProvider) discordStatus {
	if provider == nil {
		return discordStatus{Connected: false}
	}
	connected, username, latency := provider.Status()
	return discordStatus{Connected: connected, Username: username, Latency: latency}
}

// getRedisStatus retrieves redis status without exposing the underlying adapter implementation.
func getRedisStatus(services *quack.Services) redisStatus {
	if services == nil || services.Store == nil {
		return redisStatus{Connected: false}
	}

	start := time.Now()
	err := services.Store.PingRedis(context.Background())
	if err != nil {
		return redisStatus{
			Connected: false,
		}
	}
	latency := time.Since(start).Milliseconds()
	return redisStatus{
		Connected: true,
		Latency:   int64(latency),
	}
}

// getDBStatus retrieves dbstatus without exposing the underlying adapter implementation.
func getDBStatus(services *quack.Services) dbStatus {
	if services == nil || services.Store == nil {
		return dbStatus{Connected: false}
	}

	start := time.Now()
	err := services.Store.PingDatabase(context.Background())
	if err != nil {
		return dbStatus{
			Connected: false,
		}
	}
	latency := time.Since(start).Milliseconds()
	return dbStatus{
		Connected: true,
		Latency:   int64(latency),
	}
}
