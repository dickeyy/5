package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/storage"
)

func SetupRoutes(r *gin.Engine, s *storage.Store) {
	r.GET("/status", func(c *gin.Context) { status(c, s) })
}
