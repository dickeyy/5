package routes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// caseCreateRequest is the strict dashboard contract; adapter-owned reason and source cannot be supplied by callers.
type caseCreateRequest struct {
	TemplateID              string          `json:"template_id"`
	TargetDiscordUserID     string          `json:"target_discord_user_id"`
	ContextChannelDiscordID string          `json:"context_channel_discord_id"`
	ContextMessageDiscordID string          `json:"context_message_discord_id"`
	ContextURL              string          `json:"context_url"`
	Metadata                json.RawMessage `json:"metadata"`
}

// listCases returns cases subject to authorization, ordering, and filtering constraints.
func listCases(c *gin.Context, services *quack.Services) {
	result, err := services.Cases.List(c.Request.Context(), middleware.GetGuildContext(c), caseListInput(c))
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// createCase creates case while preserving validation, authorization, and persistence invariants.
func createCase(c *gin.Context, services *quack.Services) {
	var input caseCreateRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid case payload"})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid case payload"})
		return
	}

	created, err := services.Cases.Create(c.Request.Context(), middleware.GetGuildContext(c), quack.CaseInput{
		TemplateID:              input.TemplateID,
		TargetDiscordUserID:     input.TargetDiscordUserID,
		Source:                  model.CaseSourceDashboard,
		ContextChannelDiscordID: input.ContextChannelDiscordID,
		ContextMessageDiscordID: input.ContextMessageDiscordID,
		ContextURL:              input.ContextURL,
		Metadata:                input.Metadata,
	})
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"case": created})
}

// getCase retrieves case without exposing the underlying adapter implementation.
func getCase(c *gin.Context, services *quack.Services) {
	result, err := services.Cases.Get(c.Request.Context(), middleware.GetGuildContext(c), c.Param("caseRef"))
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"case": result})
}

// listUserCases returns user cases subject to authorization, ordering, and filtering constraints.
func listUserCases(c *gin.Context, services *quack.Services) {
	result, err := services.Cases.UserHistory(c.Request.Context(), middleware.GetGuildContext(c), c.Param("targetDiscordUserID"), caseListInput(c))
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// listAuditLog returns audit log subject to authorization, ordering, and filtering constraints.
func listAuditLog(c *gin.Context, services *quack.Services) {
	result, err := services.Audits.List(c.Request.Context(), middleware.GetGuildContext(c), quack.AuditListInput{
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

// caseListInput encapsulates the case list input rule so callers share one consistent package implementation.
func caseListInput(c *gin.Context) quack.CaseListInput {
	return quack.CaseListInput{
		Limit:                  c.Query("limit"),
		Offset:                 c.Query("offset"),
		TargetDiscordUserID:    c.Query("target_discord_user_id"),
		ModeratorDiscordUserID: c.Query("moderator_discord_user_id"),
		TemplateID:             c.Query("template_id"),
		Validity:               c.Query("validity"),
	}
}

// writeCaseError maps case error into the preserved HTTP error response contract.
func writeCaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrCaseValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrCasePermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrCaseTemplateNotAvailable):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrCaseNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "case operation failed"})
	}
}

// writeAuditError maps audit error into the preserved HTTP error response contract.
func writeAuditError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrAuditValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrAuditPermissionDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "audit operation failed"})
	}
}
