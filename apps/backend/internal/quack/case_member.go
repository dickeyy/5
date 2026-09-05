package quack

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// MemberCaseDetail is the privacy-safe projection available only to the target Discord identity.
type MemberCaseDetail struct {
	ID                string                     `json:"id"`
	GuildID           string                     `json:"guild_id"`
	CaseNumber        uint64                     `json:"case_number"`
	TemplateID        *string                    `json:"template_id"`
	Reason            string                     `json:"official_reason"`
	Validity          model.CaseValidity         `json:"validity"`
	VoidedReason      string                     `json:"voided_reason,omitempty"`
	ReplacementCaseID *string                    `json:"replacement_case_id,omitempty"`
	CreatedAt         time.Time                  `json:"created_at"`
	ContextValues     []CaseContextValueResponse `json:"context"`
	SelectedLevel     *CaseSelectedLevel         `json:"selected_outcome"`
	Enforcement       *MemberEnforcementOutcome  `json:"enforcement,omitempty"`
	Evidence          []CaseEvidenceResponse     `json:"evidence"`
	Events            []CaseEventResponse        `json:"history"`
	Notification      *CaseNotificationResponse  `json:"notification,omitempty"`
	Appealable        bool                       `json:"appealable"`
	AppealID          string                     `json:"appeal_id,omitempty"`
	AppealStatus      model.AppealStatus         `json:"appeal_status,omitempty"`
}

// MemberCaseSummary is the deliberately small list projection that cannot expose moderator or adapter internals.
type MemberCaseSummary struct {
	ID            string             `json:"id"`
	GuildID       string             `json:"guild_id"`
	CaseNumber    uint64             `json:"case_number"`
	Reason        string             `json:"official_reason"`
	Validity      model.CaseValidity `json:"validity"`
	CreatedAt     time.Time          `json:"created_at"`
	SelectedLevel *CaseSelectedLevel `json:"selected_outcome,omitempty"`
	Appealable    bool               `json:"appealable"`
	AppealID      string             `json:"appeal_id,omitempty"`
	AppealStatus  model.AppealStatus `json:"appeal_status,omitempty"`
}

// MemberCaseListResponse returns only target-owned privacy-safe summaries.
type MemberCaseListResponse struct {
	Cases  []MemberCaseSummary `json:"cases"`
	Total  int64               `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// MemberEnforcementOutcome exposes only the configured action and public result.
type MemberEnforcementOutcome struct {
	ActionType model.ActionType            `json:"action_type"`
	Status     model.ActionExecutionStatus `json:"status"`
}

// ListMemberCases returns only cases targeting the authenticated Discord identity and does not require current guild membership.
func (s *CaseService) ListMemberCases(ctx context.Context, guildID, memberDiscordUserID string, input CaseListInput) (*MemberCaseListResponse, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	guildID = strings.TrimSpace(guildID)
	memberDiscordUserID = strings.TrimSpace(memberDiscordUserID)
	if guildID == "" || memberDiscordUserID == "" {
		return nil, validationCaseError("guild and member identity are required")
	}
	limit, offset, err := pagination(input.Limit, input.Offset)
	if err != nil {
		return nil, err
	}
	result, err := s.store.ListCasesFiltered(ctx, model.ListCasesParams{GuildID: guildID, TargetDiscordUserID: memberDiscordUserID, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	responses := make([]MemberCaseSummary, 0, len(result.Cases))
	for _, item := range result.Cases {
		appeal, appealErr := s.store.GetAppealByCaseID(ctx, item.ID)
		if appealErr != nil {
			return nil, appealErr
		}
		appealID, appealStatus := "", model.AppealStatus("")
		if appeal != nil {
			appealID, appealStatus = appeal.ID, appeal.Status
		}
		responses = append(responses, MemberCaseSummary{ID: item.ID, GuildID: item.GuildID, CaseNumber: item.CaseNumber, Reason: item.Reason, Validity: item.Validity, CreatedAt: item.CreatedAt, SelectedLevel: selectedLevelResponse(item.TemplateSnapshotJSON), Appealable: caseSnapshotAppealable(item.TemplateSnapshotJSON) && item.Validity == model.CaseValidityValid && appeal == nil, AppealID: appealID, AppealStatus: appealStatus})
	}
	if err := s.memberReadAudit(ctx, guildID, memberDiscordUserID, "member_case.list", "guild", guildID); err != nil {
		return nil, err
	}
	return &MemberCaseListResponse{Cases: responses, Total: result.Total, Limit: limit, Offset: offset}, nil
}

// GetMemberCase returns a privacy-safe case detail only when the authenticated identity owns the case.
func (s *CaseService) GetMemberCase(ctx context.Context, caseID, memberDiscordUserID string) (*MemberCaseDetail, error) {
	item, err := s.store.GetCaseByID(ctx, strings.TrimSpace(caseID))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCaseNotFound
	}
	if item.TargetDiscordUserID != strings.TrimSpace(memberDiscordUserID) {
		requestID, correlationID := TraceIDsFromContext(ctx)
		_ = recordAudit(ctx, s.store, &model.AuditLogEntry{GuildID: item.GuildID, ActorDiscordUserID: strings.TrimSpace(memberDiscordUserID), Source: model.AuditSourceWeb, Action: "member_case.read", ResourceType: "case", ResourceID: item.ID, Result: model.AuditResultDenied, FailureReason: "not_case_target", RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"})
		return nil, ErrCaseNotFound
	}
	if err := s.memberReadAudit(ctx, item.GuildID, memberDiscordUserID, "member_case.read", "case", item.ID); err != nil {
		return nil, err
	}
	evidence, attachments, err := s.store.ListCaseEvidence(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListCaseEvents(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	publicEvents := make([]model.CaseEvent, 0, len(events))
	for _, event := range events {
		if event.Visibility == model.EventVisibilityPublic {
			event.ActorDiscordUserID = ""
			event.MetadataJSON = "{}"
			publicEvents = append(publicEvents, event)
		}
	}
	notification, err := s.store.GetCaseNotification(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	appealable := false
	if snapshot := templateSnapshotResponse(item.TemplateSnapshotJSON); snapshot != nil {
		var raw struct {
			Template struct {
				Appealable bool `json:"appealable"`
			} `json:"template"`
		}
		_ = json.Unmarshal([]byte(item.TemplateSnapshotJSON), &raw)
		appealable = raw.Template.Appealable
	}
	actions, err := s.store.ListCaseActionExecutions(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	var enforcement *MemberEnforcementOutcome
	if len(actions) > 0 {
		enforcement = &MemberEnforcementOutcome{ActionType: actions[0].ActionType, Status: actions[0].Status}
	}
	appeal, err := s.store.GetAppealByCaseID(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	appealID, appealStatus := "", model.AppealStatus("")
	if appeal != nil {
		appealID, appealStatus = appeal.ID, appeal.Status
	}
	return &MemberCaseDetail{ID: item.ID, GuildID: item.GuildID, CaseNumber: item.CaseNumber, TemplateID: item.TemplateID, Reason: item.Reason, Validity: item.Validity, VoidedReason: item.VoidedReason, ReplacementCaseID: item.ReplacementCaseID, CreatedAt: item.CreatedAt, ContextValues: parseCaseContextValues(item.ContextValuesJSON), SelectedLevel: selectedLevelResponse(item.TemplateSnapshotJSON), Enforcement: enforcement, Evidence: caseEvidenceResponses(evidence, attachments, true), Events: caseEventResponses(publicEvents), Notification: caseNotificationResponse(notification, true), Appealable: appealable && item.Validity == model.CaseValidityValid && appeal == nil, AppealID: appealID, AppealStatus: appealStatus}, nil
}

// memberReadAudit records target-owned reads without requiring a current staff or guild membership cache.
func (s *CaseService) memberReadAudit(ctx context.Context, guildID, actorID, action, resourceType, resourceID string) error {
	requestID, correlationID := TraceIDsFromContext(ctx)
	return recordAudit(ctx, s.store, &model.AuditLogEntry{GuildID: guildID, ActorDiscordUserID: actorID, Source: model.AuditSourceWeb, Action: action, ResourceType: resourceType, ResourceID: resourceID, Result: model.AuditResultSuccess, RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"})
}
