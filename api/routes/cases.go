package routes

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/api/middleware"
	"github.com/quackdiscord/bot/app"
)

func listCases(c *gin.Context, services *app.Services) {
	result, err := services.Cases.List(c.Request.Context(), middleware.GetGuildContext(c), caseListInput(c))
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func createCase(c *gin.Context, services *app.Services) {
	var input app.CaseInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid case payload"})
		return
	}

	created, err := services.Cases.Create(c.Request.Context(), middleware.GetGuildContext(c), input)
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"case": created})
}

func getCase(c *gin.Context, services *app.Services) {
	result, err := services.Cases.Get(c.Request.Context(), middleware.GetGuildContext(c), c.Param("caseRef"))
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"case": result})
}

func listUserCases(c *gin.Context, services *app.Services) {
	result, err := services.Cases.UserHistory(c.Request.Context(), middleware.GetGuildContext(c), c.Param("targetDiscordUserID"), caseListInput(c))
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func listAuditLog(c *gin.Context, services *app.Services) {
	result, err := services.Audits.List(c.Request.Context(), middleware.GetGuildContext(c), app.AuditListInput{
		Limit:              c.Query("limit"),
		Offset:             c.Query("offset"),
		ActorDiscordUserID: c.Query("actor_discord_user_id"),
		Action:             c.Query("action"),
		ResourceType:       c.Query("resource_type"),
		ResourceID:         c.Query("resource_id"),
		Result:             c.Query("result"),
	})
	if err != nil {
		writeAuditError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

func caseListInput(c *gin.Context) app.CaseListInput {
	return app.CaseListInput{
		Limit:                  c.Query("limit"),
		Offset:                 c.Query("offset"),
		TargetDiscordUserID:    c.Query("target_discord_user_id"),
		ModeratorDiscordUserID: c.Query("moderator_discord_user_id"),
		TemplateID:             c.Query("template_id"),
		Status:                 c.Query("status"),
	}
}

func writeCaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, app.ErrCaseValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, app.ErrCasePermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, app.ErrCaseTemplateNotAvailable):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, app.ErrCaseNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "case operation failed"})
	}
}

func writeAuditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, app.ErrAuditValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, app.ErrAuditPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "audit operation failed"})
	}
}
