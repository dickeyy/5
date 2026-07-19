package routes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
)

// getGuildSettings returns the guild-owned setup state to current Manage Guild authorities.
func getGuildSettings(c *gin.Context, services *quack.Services) {
	settings, err := services.Settings.Get(c.Request.Context(), middleware.GetGuildContext(c))
	if err != nil {
		writeGuildSettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// updateGuildSettings applies a partial authorized guild settings write.
func updateGuildSettings(c *gin.Context, services *quack.Services) {
	var input quack.GuildSettingsInput
	if err := bindGuildSettingsInput(c, &input); err != nil {
		writeGuildSettingsError(c, services.Settings.RejectUpdatePayload(c.Request.Context(), middleware.GetGuildContext(c), err))
		return
	}
	settings, err := services.Settings.Update(c.Request.Context(), middleware.GetGuildContext(c), input)
	if err != nil {
		writeGuildSettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// acknowledgeStarterPolicyNotice explicitly marks the one-time dashboard setup notice complete.
func acknowledgeStarterPolicyNotice(c *gin.Context, services *quack.Services) {
	settings, err := services.Settings.AcknowledgeStarterPolicyNotice(c.Request.Context(), middleware.GetGuildContext(c))
	if err != nil {
		writeGuildSettingsError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// bindGuildSettingsInput rejects unknown fields and multiple JSON documents.
func bindGuildSettingsInput(c *gin.Context, input *quack.GuildSettingsInput) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}

// writeGuildSettingsError maps service failures into stable dashboard HTTP responses.
func writeGuildSettingsError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrGuildSettingsValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrGuildSettingsPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrGuildSettingsNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "guild settings operation failed"})
	}
}
