package routes

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// caseCreateRequest is the strict dashboard contract; adapter-owned reason and source cannot be supplied by callers.
type caseCreateRequest struct {
	TemplateID              string                        `json:"template_id"`
	TargetDiscordUserID     string                        `json:"target_discord_user_id"`
	ContextChannelDiscordID string                        `json:"context_channel_discord_id"`
	ContextMessageDiscordID string                        `json:"context_message_discord_id"`
	ContextURL              string                        `json:"context_url"`
	Metadata                json.RawMessage               `json:"metadata"`
	ContextValues           []quack.CaseContextValueInput `json:"context_values"`
	EvidenceLinks           []string                      `json:"evidence_links"`
	ReplacesCaseID          string                        `json:"replaces_case_id"`
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
		ContextValues:           input.ContextValues, EvidenceLinks: input.EvidenceLinks, ReplacesCaseID: input.ReplacesCaseID, IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		writeCaseError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"case": created})
}

type voidCaseRequest struct {
	Reason            string  `json:"reason"`
	ReplacementCaseID *string `json:"replacement_case_id"`
}

// voidCase preserves the case and creates immutable correction history.
func voidCase(c *gin.Context, services *quack.Services) {
	var input voidCaseRequest
	if err := decodeStrictJSON(c, &input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid void payload"})
		return
	}
	result, err := services.Cases.Void(c.Request.Context(), middleware.GetGuildContext(c), c.Param("caseRef"), input.Reason, input.ReplacementCaseID)
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": result})
}

// listFailedActions returns the active staff recovery queue.
func listFailedActions(c *gin.Context, services *quack.Services) {
	limit, offset, err := parsePageInts(c.Query("limit"), c.Query("offset"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid pagination"})
		return
	}
	result, err := services.Actions.ListFailures(c.Request.Context(), middleware.GetGuildContext(c), limit, offset)
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// retryFailedAction requests a live-authorized retry of the same immutable action.
func retryFailedAction(c *gin.Context, services *quack.Services) {
	result, err := services.Actions.Retry(c.Request.Context(), middleware.GetGuildContext(c), c.Param("executionID"))
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"action": result})
}

// dismissFailedAction removes a failure from active review without deleting history.
func dismissFailedAction(c *gin.Context, services *quack.Services) {
	result, err := services.Actions.Dismiss(c.Request.Context(), middleware.GetGuildContext(c), c.Param("executionID"))
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"action": result})
}

type reverseActionRequest struct {
	OriginalExecutionID string           `json:"original_execution_id"`
	ActionType          model.ActionType `json:"action_type"`
	AppealID            *string          `json:"appeal_id"`
	Confirm             bool             `json:"confirm"`
}

// reverseCaseAction queues an explicitly confirmed timeout removal or unban.
func reverseCaseAction(c *gin.Context, services *quack.Services) {
	var input reverseActionRequest
	if err := decodeStrictJSON(c, &input); err != nil || !input.Confirm {
		c.JSON(http.StatusBadRequest, gin.H{"error": "confirmed reversal payload is required"})
		return
	}
	result, err := services.Actions.ReverseForAppeal(c.Request.Context(), middleware.GetGuildContext(c), c.Param("caseRef"), input.OriginalExecutionID, input.ActionType, input.AppealID)
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"action": result})
}

// listMemberOwnedCases uses the authenticated Discord identity rather than current guild membership.
func listMemberOwnedCases(c *gin.Context, services *quack.Services) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	result, err := services.Cases.ListMemberCases(c.Request.Context(), c.Param("guildID"), session.DiscordUserID, caseListInput(c))
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// getMemberOwnedCase returns the privacy-safe projection only to the target identity.
func getMemberOwnedCase(c *gin.Context, services *quack.Services) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}
	result, err := services.Cases.GetMemberCase(c.Request.Context(), c.Param("caseID"), session.DiscordUserID)
	if err != nil {
		writeCaseError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"case": result})
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
		Limit:               c.Query("limit"),
		Offset:              c.Query("offset"),
		ActorDiscordUserID:  c.Query("actor_discord_user_id"),
		Source:              c.Query("source"),
		Action:              c.Query("action"),
		ResourceType:        c.Query("resource_type"),
		ResourceID:          c.Query("resource_id"),
		Result:              c.Query("result"),
		CaseID:              c.Query("case_id"),
		MemberDiscordUserID: c.Query("member_discord_user_id"),
		CreatedAfter:        c.Query("created_after"),
		CreatedBefore:       c.Query("created_before"),
		ReadSource:          model.AuditSourceAPI,
		BeforeID:            c.Query("before_id"),
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
		CaseNumber:             c.Query("case_number"), ActionResult: c.Query("action_result"), AppealStatus: c.Query("appeal_status"), CreatedAfter: c.Query("created_after"), CreatedBefore: c.Query("created_before"),
	}
}

// decodeStrictJSON rejects unknown fields and trailing JSON values.
func decodeStrictJSON(c *gin.Context, value any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

// parsePageInts applies the shared bounded staff recovery pagination contract.
func parsePageInts(limitRaw, offsetRaw string) (int, int, error) {
	input := quack.CaseListInput{Limit: limitRaw, Offset: offsetRaw}
	limit := 50
	offset := 0
	var err error
	if input.Limit != "" {
		limit, err = strconv.Atoi(input.Limit)
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, errors.New("invalid limit")
		}
	}
	if input.Offset != "" {
		offset, err = strconv.Atoi(input.Offset)
		if err != nil || offset < 0 {
			return 0, 0, errors.New("invalid offset")
		}
	}
	return limit, offset, nil
}

// writeCaseError maps case error into the preserved HTTP error response contract.
func writeCaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, quack.ErrCaseValidation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, quack.ErrCasePermissionDenied), errors.Is(err, quack.ErrAuthorizationDenied):
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
