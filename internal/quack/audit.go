package quack

import (
	"context"
	"encoding/json"
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
	Limit               string
	Offset              string
	ActorDiscordUserID  string
	Action              string
	ResourceType        string
	ResourceID          string
	Result              string
	Source              string
	CaseID              string
	MemberDiscordUserID string
	CreatedAfter        string
	CreatedBefore       string
	ReadSource          model.AuditSource
	BeforeID            string
}

// AuditListResponse is the transport-neutral representation returned for audit list response.
type AuditListResponse struct {
	Entries    []AuditEntryResponse `json:"entries"`
	Total      int64                `json:"total"`
	Limit      int                  `json:"limit"`
	Offset     int                  `json:"offset"`
	NextCursor string               `json:"next_cursor,omitempty"`
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
		_ = s.recordRead(ctx, guildContext, input, model.AuditResultDenied, "permission_denied")
		return nil, ErrAuditPermissionDenied
	}

	limit, offset, err := auditPagination(input.Limit, input.Offset)
	if err != nil {
		return nil, err
	}
	beforeID := strings.TrimSpace(input.BeforeID)
	if beforeID != "" && len(beforeID) != 26 {
		return nil, validationAuditError("before_id must be an audit entry ID")
	}
	if beforeID != "" && offset != 0 {
		return nil, validationAuditError("offset and before_id cannot be combined")
	}
	resultValue := model.AuditResult(strings.TrimSpace(input.Result))
	if resultValue != "" && !validAuditResult(resultValue) {
		return nil, validationAuditError("result is invalid")
	}
	source := model.AuditSource(strings.TrimSpace(input.Source))
	if source != "" && !validAuditSource(source) {
		return nil, validationAuditError("source is invalid")
	}
	createdAfter, err := normalizeAuditTime(input.CreatedAfter)
	if err != nil {
		return nil, err
	}
	createdBefore, err := normalizeAuditTime(input.CreatedBefore)
	if err != nil {
		return nil, err
	}
	if strings.ContainsAny(input.CaseID, `%_\\`) || strings.ContainsAny(input.MemberDiscordUserID, `%_\\`) {
		return nil, validationAuditError("case and member filters must be exact identifiers")
	}

	result, err := s.store.ListAuditLogEntriesFiltered(ctx, model.ListAuditLogEntriesParams{
		GuildID:             guildContext.Guild.ID,
		ActorDiscordUserID:  strings.TrimSpace(input.ActorDiscordUserID),
		Source:              string(source),
		Action:              strings.TrimSpace(input.Action),
		ResourceType:        strings.TrimSpace(input.ResourceType),
		ResourceID:          strings.TrimSpace(input.ResourceID),
		Result:              resultValue,
		CaseID:              strings.TrimSpace(input.CaseID),
		MemberDiscordUserID: strings.TrimSpace(input.MemberDiscordUserID),
		CreatedAfter:        createdAfter,
		CreatedBefore:       createdBefore,
		BeforeID:            beforeID,
		Limit:               limit,
		Offset:              offset,
	})
	if err != nil {
		_ = s.recordRead(ctx, guildContext, input, model.AuditResultFailure, "query_failed")
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

	nextCursor := ""
	if len(entries) == limit {
		nextCursor = entries[len(entries)-1].ID
	}
	response := &AuditListResponse{
		Entries:    entries,
		Total:      result.Total,
		Limit:      limit,
		Offset:     offset,
		NextCursor: nextCursor,
	}
	if err := s.recordRead(ctx, guildContext, input, model.AuditResultSuccess, ""); err != nil {
		return nil, err
	}
	return response, nil
}

// recordRead appends permission-sensitive audit access without copying result data into metadata.
func (s *AuditService) recordRead(ctx context.Context, guildContext *GuildStaffContext, input AuditListInput, result model.AuditResult, failure string) error {
	if guildContext == nil || guildContext.Guild == nil {
		return nil
	}
	actorID := ""
	permissionBits := uint64(0)
	if guildContext.Staff != nil {
		actorID = guildContext.Staff.DiscordUserID
		permissionBits = guildContext.PermissionBits
	}
	requestID, correlationID := TraceIDsFromContext(ctx)
	metadata, _ := json.Marshal(map[string]any{
		"actor_filter":    strings.TrimSpace(input.ActorDiscordUserID) != "",
		"source_filter":   strings.TrimSpace(input.Source) != "",
		"action_filter":   strings.TrimSpace(input.Action) != "",
		"resource_filter": strings.TrimSpace(input.ResourceType) != "" || strings.TrimSpace(input.ResourceID) != "",
		"case_filter":     strings.TrimSpace(input.CaseID) != "",
		"member_filter":   strings.TrimSpace(input.MemberDiscordUserID) != "",
		"date_filter":     strings.TrimSpace(input.CreatedAfter) != "" || strings.TrimSpace(input.CreatedBefore) != "",
	})
	readSource := input.ReadSource
	if !validAuditSource(readSource) || readSource == "" {
		readSource = model.AuditSourceAPI
	}
	return s.store.CreateAuditLogEntry(ctx, &model.AuditLogEntry{GuildID: guildContext.Guild.ID, ActorDiscordUserID: actorID, ActorPermissionBits: permissionBits, Source: readSource, Action: string(model.AuditActionAuditRead), ResourceType: "audit_log", ResourceID: "list", Result: result, FailureReason: failure, RequestID: requestID, CorrelationID: correlationID, MetadataJSON: string(metadata)})
}

func normalizeAuditTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", validationAuditError("date filter must use RFC3339")
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func validAuditSource(source model.AuditSource) bool {
	switch source {
	case model.AuditSourceAPI, model.AuditSourceWeb, model.AuditSourceDiscord, model.AuditSourceSystem, model.AuditSourceImport, model.AuditSourceHoneypot:
		return true
	default:
		return false
	}
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
