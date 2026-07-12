package quack

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
)

var (
	ErrAuditValidation       = errors.New("audit validation failed")
	ErrAuditPermissionDenied = errors.New("audit permission denied")
)

// AuditService provides authorized, filtered access to immutable moderation audit entries.
type AuditService struct {
	store Repository
}

// AuditListInput groups the validated inputs needed for audit list input.
type AuditListInput struct {
	Limit              string
	Offset             string
	ActorDiscordUserID string
	Action             string
	ResourceType       string
	ResourceID         string
	Result             string
}

// AuditListResponse is the transport-neutral representation returned for audit list response.
type AuditListResponse struct {
	Entries []AuditEntryResponse `json:"entries"`
	Total   int64                `json:"total"`
	Limit   int                  `json:"limit"`
	Offset  int                  `json:"offset"`
}

// AuditEntryResponse is the transport-neutral representation returned for audit entry response.
type AuditEntryResponse struct {
	ID                  string            `json:"id"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
	GuildID             string            `json:"guild_id"`
	ActorDiscordUserID  string            `json:"actor_discord_user_id,omitempty"`
	ActorPermissionBits string            `json:"actor_permission_bits"`
	Source              model.AuditSource `json:"source"`
	Action              string            `json:"action"`
	ResourceType        string            `json:"resource_type"`
	ResourceID          string            `json:"resource_id"`
	Result              model.AuditResult `json:"result"`
	FailureReason       string            `json:"failure_reason,omitempty"`
	CorrelationID       string            `json:"correlation_id,omitempty"`
	RequestID           string            `json:"request_id,omitempty"`
	Metadata            any               `json:"metadata"`
}

// NewAuditService constructs audit service with required dependencies explicit so callers control lifecycle and substitution.
func NewAuditService(store Repository) *AuditService {
	return &AuditService{store: store}
}

// List returns list subject to authorization, ordering, and filtering constraints.
func (s *AuditService) List(ctx context.Context, guildContext *GuildStaffContext, input AuditListInput) (*AuditListResponse, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("audit service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, validationAuditError("missing guild context")
	}
	if !guildContext.Can(model.PermissionActionAuditRead) {
		return nil, ErrAuditPermissionDenied
	}

	limit, offset, err := auditPagination(input.Limit, input.Offset)
	if err != nil {
		return nil, err
	}
	resultValue := model.AuditResult(strings.TrimSpace(input.Result))
	if resultValue != "" && !validAuditResult(resultValue) {
		return nil, validationAuditError("result is invalid")
	}

	result, err := s.store.ListAuditLogEntriesFiltered(ctx, model.ListAuditLogEntriesParams{
		GuildID:            guildContext.Guild.ID,
		ActorDiscordUserID: strings.TrimSpace(input.ActorDiscordUserID),
		Action:             strings.TrimSpace(input.Action),
		ResourceType:       strings.TrimSpace(input.ResourceType),
		ResourceID:         strings.TrimSpace(input.ResourceID),
		Result:             resultValue,
		Limit:              limit,
		Offset:             offset,
	})
	if err != nil {
		return nil, err
	}

	entries := make([]AuditEntryResponse, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, AuditEntryResponse{
			ID:                  entry.ID,
			CreatedAt:           entry.CreatedAt,
			UpdatedAt:           entry.UpdatedAt,
			GuildID:             entry.GuildID,
			ActorDiscordUserID:  entry.ActorDiscordUserID,
			ActorPermissionBits: PermissionBitsString(entry.ActorPermissionBits),
			Source:              entry.Source,
			Action:              entry.Action,
			ResourceType:        entry.ResourceType,
			ResourceID:          entry.ResourceID,
			Result:              entry.Result,
			FailureReason:       entry.FailureReason,
			CorrelationID:       entry.CorrelationID,
			RequestID:           entry.RequestID,
			Metadata:            parseJSON(entry.MetadataJSON),
		})
	}

	return &AuditListResponse{
		Entries: entries,
		Total:   result.Total,
		Limit:   limit,
		Offset:  offset,
	}, nil
}

// auditPagination records audit pagination so moderation changes remain attributable.
func auditPagination(limitValue, offsetValue string) (int, int, error) {
	limit := 50
	if strings.TrimSpace(limitValue) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(limitValue))
		if err != nil || parsed <= 0 {
			return 0, 0, validationAuditError("limit must be a positive integer")
		}
		limit = parsed
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if strings.TrimSpace(offsetValue) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(offsetValue))
		if err != nil || parsed < 0 {
			return 0, 0, validationAuditError("offset must be a non-negative integer")
		}
		offset = parsed
	}

	return limit, offset, nil
}

// validAuditResult checks valid audit result before state is read or changed.
func validAuditResult(result model.AuditResult) bool {
	switch result {
	case model.AuditResultSuccess, model.AuditResultFailure, model.AuditResultDenied:
		return true
	default:
		return false
	}
}

// validationAuditError checks validation audit error before state is read or changed.
func validationAuditError(message string) error {
	return fmt.Errorf("%w: %s", ErrAuditValidation, message)
}
