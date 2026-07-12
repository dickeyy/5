package routes

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// RegisterAuditStatisticsStaffRoutes mounts QP-E feature routes without editing the integration-owned router.
func RegisterAuditStatisticsStaffRoutes(group *gin.RouterGroup, services *quack.Services) {
	statistics := quack.NewStaffStatisticsService(services.Store)
	group.GET("/:discordGuildID/statistics", middleware.RequireGuildContext(services, model.PermissionActionAuditRead), func(c *gin.Context) {
		result, err := statistics.Get(c.Request.Context(), middleware.GetGuildContext(c), quack.StatisticsInput{From: c.Query("from"), To: c.Query("to")})
		if err != nil {
			writeStatisticsError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
	})
}

func writeStatisticsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrStatisticsValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrStatisticsPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": "statistics access denied"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "statistics operation failed"})
	}
}
