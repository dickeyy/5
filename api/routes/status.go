package routes

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/discord"
	"github.com/quackdiscord/bot/services"
)

type discordStatus struct {
	Connected bool   `json:"connected"`
	Username  string `json:"username,omitempty"`
	Latency   int64  `json:"latency,omitempty"`
}

type redisStatus struct {
	Connected bool  `json:"connected"`
	Latency   int64 `json:"latency,omitempty"`
}

type dbStatus struct {
	Connected bool  `json:"connected"`
	Latency   int64 `json:"latency,omitempty"`
}

func status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"discord":  getDiscordStatus(),
		"redis":    getRedisStatus(),
		"database": getDBStatus(),
	})
}

func getDiscordStatus() discordStatus {
	user := discord.Session.State.User
	if user != nil {
		return discordStatus{
			Connected: true,
			Username:  user.Username,
			Latency:   discord.Session.HeartbeatLatency().Milliseconds(),
		}
	}
	return discordStatus{
		Connected: false,
	}
}

func getRedisStatus() redisStatus {
	start := time.Now()
	_, err := services.Redis.Ping(context.Background()).Result()
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

func getDBStatus() dbStatus {
	start := time.Now()
	err := services.DB.Ping()
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
