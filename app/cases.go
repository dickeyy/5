package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

var (
	ErrCaseValidation           = errors.New("case validation failed")
	ErrCaseTemplateNotAvailable = errors.New("case template not available")
	ErrCasePermissionDenied     = errors.New("case permission denied")
)

type CaseService struct {
	store *storage.Store
}

type CaseInput struct {
	TemplateID              string             `json:"template_id"`
	TargetDiscordUserID     string             `json:"target_discord_user_id"`
	ReasonOverride          string             `json:"reason_override"`
	Source                  structs.CaseSource `json:"source"`
	ContextChannelDiscordID string             `json:"context_channel_discord_id"`
	ContextMessageDiscordID string             `json:"context_message_discord_id"`
	ContextURL              string             `json:"context_url"`
	Metadata                json.RawMessage    `json:"metadata"`
}

type CaseResponse struct {
	ID                     string               `json:"id"`
	GuildID                string               `json:"guild_id"`
	CaseNumber             uint64               `json:"case_number"`
	TemplateID             *string              `json:"template_id"`
	TemplateVersion        uint                 `json:"template_version"`
	TargetDiscordUserID    string               `json:"target_discord_user_id"`
	ModeratorDiscordUserID string               `json:"moderator_discord_user_id"`
	Reason                 string               `json:"reason"`
	Severity               structs.CaseSeverity `json:"severity"`
	Weight                 int                  `json:"weight"`
	Status                 structs.CaseStatus   `json:"status"`
	Source                 structs.CaseSource   `json:"source"`
	SelectedLevel          *CaseSelectedLevel   `json:"selected_level,omitempty"`
	Actions                []CaseActionResponse `json:"actions"`
}

type CaseSelectedLevel struct {
	TemplateLevelDetails
	MatchedCaseCount int64 `json:"matched_case_count"`
}

type CaseActionResponse struct {
	ID               string                        `json:"id"`
	Position         int                           `json:"position"`
	ActionType       structs.ActionType            `json:"action_type"`
	Status           structs.ActionExecutionStatus `json:"status"`
	TemplateActionID *string                       `json:"template_action_id"`
	IdempotencyKey   string                        `json:"idempotency_key"`
	NotifyUser       bool                          `json:"notify_user"`
	NotificationType string                        `json:"notification_type,omitempty"`
	MaxRetries       uint8                         `json:"max_retries"`
	RetryBackoffMS   int                           `json:"retry_backoff_ms"`
	SafeForRetry     bool                          `json:"safe_for_retry"`
	Irreversible     bool                          `json:"irreversible"`
}

type templateSnapshot struct {
	Template      templateSnapshotTemplate `json:"template"`
	SelectedLevel CaseSelectedLevel        `json:"selected_level"`
	Actions       []templateSnapshotAction `json:"actions"`
}

type templateSnapshotTemplate struct {
	ID              string               `json:"id"`
	Slug            string               `json:"slug"`
	Name            string               `json:"name"`
	Version         uint                 `json:"version"`
	ReasonTemplate  string               `json:"reason_template"`
	DefaultSeverity structs.CaseSeverity `json:"default_severity"`
}

type templateSnapshotAction struct {
	ID               string             `json:"id"`
	Position         int                `json:"position"`
	ActionType       structs.ActionType `json:"action_type"`
	Config           any                `json:"config"`
	NotifyUser       bool               `json:"notify_user"`
	NotificationType string             `json:"notification_type,omitempty"`
	ContinueOnError  bool               `json:"continue_on_error"`
	MaxRetries       uint8              `json:"max_retries"`
	RetryBackoffMS   int                `json:"retry_backoff_ms"`
	TimeoutMS        int                `json:"timeout_ms"`
	IdempotencyScope string             `json:"idempotency_scope"`
}

type selectedTemplateLevel struct {
	Level            structs.CaseTemplateLevel
	Actions          []structs.CaseTemplateLevelAction
	MatchedCaseCount int64
}

func NewCaseService(store *storage.Store) *CaseService {
	return &CaseService{store: store}
}

func (s *CaseService) Create(ctx context.Context, guildContext *GuildStaffContext, input CaseInput) (*CaseResponse, error) {
	created, err := s.create(ctx, guildContext, input)
	if err != nil {
		if errors.Is(err, ErrCaseValidation) || errors.Is(err, ErrCasePermissionDenied) || errors.Is(err, ErrCaseTemplateNotAvailable) {
			_ = s.audit(ctx, guildContext, "case.create", "case", "unknown", structs.AuditResultFailure, err.Error())
		}
		return nil, err
	}

	enqueueCaseActions(created.Case.ID)

	response := caseResponse(*created)
	return &response, nil
}

func (s *CaseService) create(ctx context.Context, guildContext *GuildStaffContext, input CaseInput) (*storage.CreatedCase, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, validationCaseError("missing guild context")
	}
	if !guildContext.Can(structs.PermissionActionCaseCreate) {
		return nil, ErrCasePermissionDenied
	}

	templateID := strings.TrimSpace(input.TemplateID)
	if templateID == "" {
		return nil, validationCaseError("template_id is required")
	}
	targetDiscordUserID := strings.TrimSpace(input.TargetDiscordUserID)
	if targetDiscordUserID == "" {
		return nil, validationCaseError("target_discord_user_id is required")
	}

	source := input.Source
	if source == "" {
		source = structs.CaseSourceAPI
	}
	if !validCaseSource(source) {
		return nil, validationCaseError("source is invalid")
	}

	metadataJSON, err := normalizeJSONObject(input.Metadata)
	if err != nil {
		return nil, validationCaseError("metadata must be a JSON object")
	}

	template, err := s.store.GetCaseTemplateExpanded(ctx, guildContext.Guild.ID, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil || !template.Template.Enabled || template.Template.ArchivedAt != nil {
		return nil, ErrCaseTemplateNotAvailable
	}

	reason := strings.TrimSpace(input.ReasonOverride)
	if reason == "" {
		reason = strings.TrimSpace(template.Template.ReasonTemplate)
	}
	if reason == "" {
		return nil, validationCaseError("reason is required")
	}

	selectedLevel, err := s.selectTemplateLevel(ctx, guildContext.Guild.ID, targetDiscordUserID, template)
	if err != nil {
		return nil, err
	}

	snapshotJSON, err := buildTemplateSnapshot(template.Template, *selectedLevel)
	if err != nil {
		return nil, err
	}

	caseModel := structs.Case{
		GuildID:                 guildContext.Guild.ID,
		TemplateID:              &template.Template.ID,
		TemplateVersion:         template.Template.Version,
		TemplateSnapshotJSON:    snapshotJSON,
		TargetDiscordUserID:     targetDiscordUserID,
		ModeratorDiscordUserID:  guildContext.Staff.DiscordUserID,
		Reason:                  reason,
		Severity:                template.Template.DefaultSeverity,
		Weight:                  1,
		Status:                  structs.CaseStatusOpen,
		Source:                  source,
		ContextChannelDiscordID: strings.TrimSpace(input.ContextChannelDiscordID),
		ContextMessageDiscordID: strings.TrimSpace(input.ContextMessageDiscordID),
		ContextURL:              strings.TrimSpace(input.ContextURL),
		MetadataJSON:            metadataJSON,
	}

	actionExecutions := make([]structs.CaseActionExecution, 0, len(selectedLevel.Actions))
	for _, action := range selectedLevel.Actions {
		templateActionID := action.ID
		actionExecutions = append(actionExecutions, structs.CaseActionExecution{
			TemplateActionID:   &templateActionID,
			Position:           action.Position,
			ActionType:         action.ActionType,
			Status:             structs.ActionExecutionPending,
			ConfigSnapshotJSON: action.ConfigJSON,
			NotifyUser:         action.NotifyUser,
			NotificationType:   action.NotificationType,
			MaxRetries:         action.MaxRetries,
			RetryBackoffMS:     action.RetryBackoffMS,
			SafeForRetry:       !irreversibleAction(action.ActionType),
			Irreversible:       irreversibleAction(action.ActionType),
		})
	}

	event := structs.CaseEvent{
		EventType:          structs.CaseEventCreated,
		ActorDiscordUserID: guildContext.Staff.DiscordUserID,
		ActorType:          "staff",
		Visibility:         structs.EventVisibilityStaff,
		Body:               fmt.Sprintf("Case created from template %s", template.Template.Slug),
		MetadataJSON:       "{}",
	}

	return s.store.CreateCase(ctx, storage.CreateCaseParams{
		Case:             caseModel,
		Event:            event,
		ActionExecutions: actionExecutions,
		Audit:            s.auditEntry(guildContext, "case.create", "case", "", structs.AuditResultSuccess, ""),
	})
}

func (s *CaseService) selectTemplateLevel(ctx context.Context, guildID, targetDiscordUserID string, template *storage.ExpandedCaseTemplate) (*selectedTemplateLevel, error) {
	if template == nil {
		return nil, validationCaseError("template is required")
	}

	var fallback *selectedTemplateLevel
	var best *selectedTemplateLevel
	now := time.Now().UTC()

	for _, expandedLevel := range template.Levels {
		level := expandedLevel.Level
		if !level.Enabled {
			continue
		}

		enabledActions := enabledLevelActions(expandedLevel.Actions)
		if level.IsDefault {
			if len(enabledActions) == 0 {
				return nil, validationCaseError("default level has no enabled actions")
			}
			matchedCaseCount, err := s.matchingTemplateCaseCount(ctx, guildID, targetDiscordUserID, template.Template.ID, nil)
			if err != nil {
				return nil, err
			}
			fallback = &selectedTemplateLevel{
				Level:            level,
				Actions:          enabledActions,
				MatchedCaseCount: matchedCaseCount,
			}
			continue
		}

		if level.TriggerCaseCount <= 0 {
			return nil, validationCaseError("escalation level trigger_case_count must be positive")
		}

		var since *time.Time
		if level.WindowMinutes > 0 {
			value := now.Add(-time.Duration(level.WindowMinutes) * time.Minute)
			since = &value
		}
		matchedCaseCount, err := s.matchingTemplateCaseCount(ctx, guildID, targetDiscordUserID, template.Template.ID, since)
		if err != nil {
			return nil, err
		}
		if matchedCaseCount < int64(level.TriggerCaseCount) {
			continue
		}
		candidate := &selectedTemplateLevel{
			Level:            level,
			Actions:          enabledActions,
			MatchedCaseCount: matchedCaseCount,
		}
		if best == nil ||
			level.TriggerCaseCount > best.Level.TriggerCaseCount ||
			(level.TriggerCaseCount == best.Level.TriggerCaseCount && level.Position > best.Level.Position) {
			best = candidate
		}
	}

	if fallback == nil {
		return nil, validationCaseError("template has no enabled default level")
	}
	if best != nil {
		if len(best.Actions) == 0 {
			return nil, validationCaseError("selected level has no enabled actions")
		}
		return best, nil
	}

	return fallback, nil
}

func (s *CaseService) matchingTemplateCaseCount(ctx context.Context, guildID, targetDiscordUserID, templateID string, since *time.Time) (int64, error) {
	priorCount, err := s.store.CountTemplateCasesForTarget(ctx, storage.CountTemplateCasesForTargetParams{
		GuildID:             guildID,
		TemplateID:          templateID,
		TargetDiscordUserID: targetDiscordUserID,
		Since:               since,
	})
	if err != nil {
		return 0, err
	}
	return priorCount + 1, nil
}

func buildTemplateSnapshot(template structs.CaseTemplate, selectedLevel selectedTemplateLevel) (string, error) {
	snapshot := templateSnapshot{
		Template: templateSnapshotTemplate{
			ID:              template.ID,
			Slug:            template.Slug,
			Name:            template.Name,
			Version:         template.Version,
			ReasonTemplate:  template.ReasonTemplate,
			DefaultSeverity: template.DefaultSeverity,
		},
		SelectedLevel: CaseSelectedLevel{
			TemplateLevelDetails: templateLevelDetails(selectedLevel.Level),
			MatchedCaseCount:     selectedLevel.MatchedCaseCount,
		},
		Actions: make([]templateSnapshotAction, 0, len(selectedLevel.Actions)),
	}

	for _, action := range selectedLevel.Actions {
		snapshot.Actions = append(snapshot.Actions, templateSnapshotAction{
			ID:               action.ID,
			Position:         action.Position,
			ActionType:       action.ActionType,
			Config:           parseJSON(action.ConfigJSON),
			NotifyUser:       action.NotifyUser,
			NotificationType: action.NotificationType,
			ContinueOnError:  action.ContinueOnError,
			MaxRetries:       action.MaxRetries,
			RetryBackoffMS:   action.RetryBackoffMS,
			TimeoutMS:        action.TimeoutMS,
			IdempotencyScope: action.IdempotencyScope,
		})
	}

	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("marshal case template snapshot: %w", err)
	}
	return string(body), nil
}

func caseResponse(created storage.CreatedCase) CaseResponse {
	response := CaseResponse{
		ID:                     created.Case.ID,
		GuildID:                created.Case.GuildID,
		CaseNumber:             created.Case.CaseNumber,
		TemplateID:             created.Case.TemplateID,
		TemplateVersion:        created.Case.TemplateVersion,
		TargetDiscordUserID:    created.Case.TargetDiscordUserID,
		ModeratorDiscordUserID: created.Case.ModeratorDiscordUserID,
		Reason:                 created.Case.Reason,
		Severity:               created.Case.Severity,
		Weight:                 created.Case.Weight,
		Status:                 created.Case.Status,
		Source:                 created.Case.Source,
		SelectedLevel:          selectedLevelResponse(created.Case.TemplateSnapshotJSON),
		Actions:                make([]CaseActionResponse, 0, len(created.ActionExecutions)),
	}

	for _, action := range created.ActionExecutions {
		response.Actions = append(response.Actions, CaseActionResponse{
			ID:               action.ID,
			Position:         action.Position,
			ActionType:       action.ActionType,
			Status:           action.Status,
			TemplateActionID: action.TemplateActionID,
			IdempotencyKey:   action.IdempotencyKey,
			NotifyUser:       action.NotifyUser,
			NotificationType: action.NotificationType,
			MaxRetries:       action.MaxRetries,
			RetryBackoffMS:   action.RetryBackoffMS,
			SafeForRetry:     action.SafeForRetry,
			Irreversible:     action.Irreversible,
		})
	}

	return response
}

func selectedLevelResponse(snapshotJSON string) *CaseSelectedLevel {
	var snapshot struct {
		SelectedLevel CaseSelectedLevel `json:"selected_level"`
	}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil || snapshot.SelectedLevel.ID == "" {
		return nil
	}
	return &snapshot.SelectedLevel
}

func validationCaseError(message string) error {
	return fmt.Errorf("%w: %s", ErrCaseValidation, message)
}

func validCaseSource(source structs.CaseSource) bool {
	switch source {
	case structs.CaseSourceAPI, structs.CaseSourceDiscordCommand, structs.CaseSourceAutomation, structs.CaseSourceImport:
		return true
	default:
		return false
	}
}

func irreversibleAction(actionType structs.ActionType) bool {
	switch actionType {
	case structs.ActionTimeoutUser, structs.ActionKickUser, structs.ActionBanUser:
		return true
	default:
		return false
	}
}

func (s *CaseService) audit(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result structs.AuditResult, failureReason string) error {
	entry := s.auditEntry(guildContext, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return s.store.CreateAuditLogEntry(ctx, entry)
}

func (s *CaseService) auditEntry(guildContext *GuildStaffContext, action, resourceType, resourceID string, result structs.AuditResult, failureReason string) *structs.AuditLogEntry {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil
	}

	entry := &structs.AuditLogEntry{
		GuildID:             guildContext.Guild.ID,
		ActorDiscordUserID:  guildContext.Staff.DiscordUserID,
		ActorPermissionBits: guildContext.PermissionBits,
		Source:              structs.AuditSourceAPI,
		Action:              action,
		ResourceType:        resourceType,
		ResourceID:          resourceID,
		Result:              result,
		FailureReason:       failureReason,
		MetadataJSON:        "{}",
	}
	if entry.ResourceID == "" {
		entry.ResourceID = "unknown"
	}
	return entry
}
