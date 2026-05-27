package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

var (
	ErrCaseValidation           = errors.New("case validation failed")
	ErrCaseTemplateNotAvailable = errors.New("case template not available")
	ErrCasePermissionDenied     = errors.New("case permission denied")
	ErrCaseNotFound             = errors.New("case not found")
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

type CaseListInput struct {
	Limit                  string
	Offset                 string
	TargetDiscordUserID    string
	ModeratorDiscordUserID string
	TemplateID             string
	Status                 string
}

type CaseListResponse struct {
	Cases  []CaseResponse `json:"cases"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type CaseDetailResponse struct {
	CaseResponse
	TemplateSnapshot *CaseTemplateSnapshotResponse `json:"template_snapshot,omitempty"`
	Actions          []CaseActionDetailResponse    `json:"actions"`
	Events           []CaseEventResponse           `json:"events"`
}

type CaseProfileResponse struct {
	Cases   []CaseResponse     `json:"cases"`
	Total   int64              `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	Summary CaseProfileSummary `json:"summary"`
}

type CaseProfileSummary struct {
	Total      int64            `json:"total"`
	ByStatus   map[string]int64 `json:"by_status"`
	ByTemplate map[string]int64 `json:"by_template"`
}

type CaseResponse struct {
	CreatedAt               time.Time            `json:"created_at"`
	UpdatedAt               time.Time            `json:"updated_at"`
	ID                      string               `json:"id"`
	GuildID                 string               `json:"guild_id"`
	CaseNumber              uint64               `json:"case_number"`
	TemplateID              *string              `json:"template_id"`
	TemplateVersion         uint                 `json:"template_version"`
	TargetDiscordUserID     string               `json:"target_discord_user_id"`
	ModeratorDiscordUserID  string               `json:"moderator_discord_user_id"`
	Reason                  string               `json:"reason"`
	Severity                structs.CaseSeverity `json:"severity"`
	Weight                  int                  `json:"weight"`
	Status                  structs.CaseStatus   `json:"status"`
	Source                  structs.CaseSource   `json:"source"`
	ContextChannelDiscordID string               `json:"context_channel_discord_id,omitempty"`
	ContextMessageDiscordID string               `json:"context_message_discord_id,omitempty"`
	ContextURL              string               `json:"context_url,omitempty"`
	ResolvedAt              *time.Time           `json:"resolved_at,omitempty"`
	ResolvedByDiscordUserID string               `json:"resolved_by_discord_user_id,omitempty"`
	Metadata                any                  `json:"metadata"`
	SelectedLevel           *CaseSelectedLevel   `json:"selected_level,omitempty"`
	Actions                 []CaseActionResponse `json:"actions"`
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

type CaseActionDetailResponse struct {
	CaseActionResponse
	ConfigSnapshot any                         `json:"config_snapshot"`
	AttemptCount   uint8                       `json:"attempt_count"`
	LastErrorCode  string                      `json:"last_error_code,omitempty"`
	LastError      string                      `json:"last_error,omitempty"`
	StartedAt      *time.Time                  `json:"started_at,omitempty"`
	FinishedAt     *time.Time                  `json:"finished_at,omitempty"`
	NextRetryAt    *time.Time                  `json:"next_retry_at,omitempty"`
	Attempts       []CaseActionAttemptResponse `json:"attempts"`
}

type CaseActionAttemptResponse struct {
	ID              string                      `json:"id"`
	ExecutionID     string                      `json:"execution_id"`
	AttemptNumber   uint8                       `json:"attempt_number"`
	Status          structs.ActionAttemptStatus `json:"status"`
	WorkerID        string                      `json:"worker_id,omitempty"`
	StartedAt       time.Time                   `json:"started_at"`
	FinishedAt      *time.Time                  `json:"finished_at,omitempty"`
	DurationMS      int64                       `json:"duration_ms"`
	ErrorCode       string                      `json:"error_code,omitempty"`
	ErrorMessage    string                      `json:"error_message,omitempty"`
	RequestPayload  any                         `json:"request_payload"`
	ResponsePayload any                         `json:"response_payload"`
}

type CaseEventResponse struct {
	ID                 string                  `json:"id"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
	EventType          structs.CaseEventType   `json:"event_type"`
	ActorDiscordUserID string                  `json:"actor_discord_user_id,omitempty"`
	ActorType          string                  `json:"actor_type"`
	Visibility         structs.EventVisibility `json:"visibility"`
	Body               string                  `json:"body"`
	Metadata           any                     `json:"metadata"`
	EditedAt           *time.Time              `json:"edited_at,omitempty"`
}

type CaseTemplateSnapshotResponse struct {
	Template      templateSnapshotTemplate `json:"template"`
	SelectedLevel CaseSelectedLevel        `json:"selected_level"`
	Actions       []templateSnapshotAction `json:"actions"`
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
	ctx = ensureTraceContext(ctx)
	created, err := s.create(ctx, guildContext, input)
	if err != nil {
		if errors.Is(err, ErrCaseValidation) || errors.Is(err, ErrCasePermissionDenied) || errors.Is(err, ErrCaseTemplateNotAvailable) {
			_ = s.audit(ctx, guildContext, "case.create", "case", "unknown", structs.AuditResultFailure, err.Error())
		}
		return nil, err
	}

	enqueueCaseActions(ctx, created.Case.ID)

	response := caseResponse(*created)
	return &response, nil
}

func (s *CaseService) List(ctx context.Context, guildContext *GuildStaffContext, input CaseListInput) (*CaseListResponse, error) {
	params, limit, offset, err := s.caseListParams(guildContext, input)
	if err != nil {
		return nil, err
	}

	result, err := s.store.ListCasesFiltered(ctx, params)
	if err != nil {
		return nil, err
	}

	responses, err := s.caseResponsesForModels(ctx, result.Cases)
	if err != nil {
		return nil, err
	}

	return &CaseListResponse{
		Cases:  responses,
		Total:  result.Total,
		Limit:  limit,
		Offset: offset,
	}, nil
}

func (s *CaseService) Get(ctx context.Context, guildContext *GuildStaffContext, caseRef string) (*CaseDetailResponse, error) {
	if err := s.requireCaseRead(guildContext); err != nil {
		return nil, err
	}
	caseRef = strings.TrimSpace(caseRef)
	if caseRef == "" {
		return nil, validationCaseError("case reference is required")
	}

	caseModel, err := s.store.GetCaseByIDOrNumber(ctx, guildContext.Guild.ID, caseRef)
	if err != nil {
		return nil, err
	}
	if caseModel == nil {
		return nil, ErrCaseNotFound
	}

	actions, err := s.store.ListCaseActionExecutions(ctx, caseModel.ID)
	if err != nil {
		return nil, err
	}
	events, err := s.store.ListCaseEvents(ctx, caseModel.ID)
	if err != nil {
		return nil, err
	}

	executionIDs := make([]string, 0, len(actions))
	for _, action := range actions {
		executionIDs = append(executionIDs, action.ID)
	}
	attempts, err := s.store.ListCaseActionAttempts(ctx, executionIDs)
	if err != nil {
		return nil, err
	}

	base := caseResponseFromModel(*caseModel, actions)
	return &CaseDetailResponse{
		CaseResponse:     base,
		TemplateSnapshot: templateSnapshotResponse(caseModel.TemplateSnapshotJSON),
		Actions:          caseActionDetailResponses(actions, attempts),
		Events:           caseEventResponses(events),
	}, nil
}

func (s *CaseService) UserHistory(ctx context.Context, guildContext *GuildStaffContext, targetDiscordUserID string, input CaseListInput) (*CaseProfileResponse, error) {
	targetDiscordUserID = strings.TrimSpace(targetDiscordUserID)
	if targetDiscordUserID == "" {
		return nil, validationCaseError("target discord user id is required")
	}
	input.TargetDiscordUserID = targetDiscordUserID

	list, err := s.List(ctx, guildContext, input)
	if err != nil {
		return nil, err
	}
	summary, err := s.store.TargetCaseSummary(ctx, guildContext.Guild.ID, targetDiscordUserID)
	if err != nil {
		return nil, err
	}

	return &CaseProfileResponse{
		Cases:  list.Cases,
		Total:  list.Total,
		Limit:  list.Limit,
		Offset: list.Offset,
		Summary: CaseProfileSummary{
			Total:      summary.Total,
			ByStatus:   caseStatusSummary(summary.ByStatus),
			ByTemplate: summary.ByTemplate,
		},
	}, nil
}

func (s *CaseService) caseListParams(guildContext *GuildStaffContext, input CaseListInput) (storage.ListCasesParams, int, int, error) {
	if err := s.requireCaseRead(guildContext); err != nil {
		return storage.ListCasesParams{}, 0, 0, err
	}

	limit, offset, err := pagination(input.Limit, input.Offset)
	if err != nil {
		return storage.ListCasesParams{}, 0, 0, err
	}

	status := structs.CaseStatus(strings.TrimSpace(input.Status))
	if status != "" && !validCaseStatus(status) {
		return storage.ListCasesParams{}, 0, 0, validationCaseError("status is invalid")
	}

	return storage.ListCasesParams{
		GuildID:                guildContext.Guild.ID,
		TargetDiscordUserID:    strings.TrimSpace(input.TargetDiscordUserID),
		ModeratorDiscordUserID: strings.TrimSpace(input.ModeratorDiscordUserID),
		TemplateID:             strings.TrimSpace(input.TemplateID),
		Status:                 status,
		Limit:                  limit,
		Offset:                 offset,
	}, limit, offset, nil
}

func (s *CaseService) requireCaseRead(guildContext *GuildStaffContext) error {
	if s == nil || s.store == nil {
		return errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return validationCaseError("missing guild context")
	}
	if !guildContext.Can(structs.PermissionActionCaseCreate) {
		return ErrCasePermissionDenied
	}
	return nil
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
	_, correlationID := TraceIDsFromContext(ctx)

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
		CorrelationID:           correlationID,
		ContextChannelDiscordID: strings.TrimSpace(input.ContextChannelDiscordID),
		ContextMessageDiscordID: strings.TrimSpace(input.ContextMessageDiscordID),
		ContextURL:              strings.TrimSpace(input.ContextURL),
		MetadataJSON:            metadataJSON,
	}

	actionExecutions := make([]structs.CaseActionExecution, 0, len(selectedLevel.Actions)+1)
	if selectedLevel.Level.NotifyUser {
		actionExecutions = append(actionExecutions, warningNotificationExecution(caseModel, selectedLevel.Level.NotificationType))
	}
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
			CorrelationID:      correlationID,
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
		Audit:            s.auditEntry(ctx, guildContext, "case.create", "case", "", structs.AuditResultSuccess, ""),
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

func warningNotificationExecution(caseModel structs.Case, notificationType string) structs.CaseActionExecution {
	if strings.TrimSpace(notificationType) == "" {
		notificationType = string(structs.NotificationWarning)
	}
	return structs.CaseActionExecution{
		Position:           0,
		ActionType:         structs.ActionSendDM,
		Status:             structs.ActionExecutionPending,
		ConfigSnapshotJSON: warningNotificationConfig(caseModel.Reason),
		NotificationType:   notificationType,
		SafeForRetry:       true,
		CorrelationID:      caseModel.CorrelationID,
	}
}

func warningNotificationConfig(reason string) string {
	body, err := json.Marshal(map[string]any{
		"message": fmt.Sprintf("You received a warning in this server: %s", reason),
	})
	if err != nil {
		return "{}"
	}
	return string(body)
}

func caseResponse(created storage.CreatedCase) CaseResponse {
	return caseResponseFromModel(created.Case, created.ActionExecutions)
}

func caseResponseFromModel(caseModel structs.Case, actionExecutions []structs.CaseActionExecution) CaseResponse {
	response := CaseResponse{
		CreatedAt:               caseModel.CreatedAt,
		UpdatedAt:               caseModel.UpdatedAt,
		ID:                      caseModel.ID,
		GuildID:                 caseModel.GuildID,
		CaseNumber:              caseModel.CaseNumber,
		TemplateID:              caseModel.TemplateID,
		TemplateVersion:         caseModel.TemplateVersion,
		TargetDiscordUserID:     caseModel.TargetDiscordUserID,
		ModeratorDiscordUserID:  caseModel.ModeratorDiscordUserID,
		Reason:                  caseModel.Reason,
		Severity:                caseModel.Severity,
		Weight:                  caseModel.Weight,
		Status:                  caseModel.Status,
		Source:                  caseModel.Source,
		ContextChannelDiscordID: caseModel.ContextChannelDiscordID,
		ContextMessageDiscordID: caseModel.ContextMessageDiscordID,
		ContextURL:              caseModel.ContextURL,
		ResolvedAt:              caseModel.ResolvedAt,
		ResolvedByDiscordUserID: caseModel.ResolvedByDiscordUserID,
		Metadata:                parseJSON(caseModel.MetadataJSON),
		SelectedLevel:           selectedLevelResponse(caseModel.TemplateSnapshotJSON),
		Actions:                 make([]CaseActionResponse, 0, len(actionExecutions)),
	}

	for _, action := range actionExecutions {
		response.Actions = append(response.Actions, caseActionResponse(action))
	}

	return response
}

func (s *CaseService) caseResponsesForModels(ctx context.Context, cases []structs.Case) ([]CaseResponse, error) {
	responses := make([]CaseResponse, 0, len(cases))
	for _, caseModel := range cases {
		actions, err := s.store.ListCaseActionExecutions(ctx, caseModel.ID)
		if err != nil {
			return nil, err
		}
		responses = append(responses, caseResponseFromModel(caseModel, actions))
	}
	return responses, nil
}

func caseActionResponse(action structs.CaseActionExecution) CaseActionResponse {
	return CaseActionResponse{
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
	}
}

func caseActionDetailResponses(actions []structs.CaseActionExecution, attempts []structs.CaseActionAttempt) []CaseActionDetailResponse {
	attemptsByExecution := map[string][]CaseActionAttemptResponse{}
	for _, attempt := range attempts {
		attemptsByExecution[attempt.ExecutionID] = append(attemptsByExecution[attempt.ExecutionID], CaseActionAttemptResponse{
			ID:              attempt.ID,
			ExecutionID:     attempt.ExecutionID,
			AttemptNumber:   attempt.AttemptNumber,
			Status:          attempt.Status,
			WorkerID:        attempt.WorkerID,
			StartedAt:       attempt.StartedAt,
			FinishedAt:      attempt.FinishedAt,
			DurationMS:      attempt.DurationMS,
			ErrorCode:       attempt.ErrorCode,
			ErrorMessage:    attempt.ErrorMessage,
			RequestPayload:  parseJSON(attempt.RequestPayloadJSON),
			ResponsePayload: parseJSON(attempt.ResponsePayloadJSON),
		})
	}

	responses := make([]CaseActionDetailResponse, 0, len(actions))
	for _, action := range actions {
		responses = append(responses, CaseActionDetailResponse{
			CaseActionResponse: caseActionResponse(action),
			ConfigSnapshot:     parseJSON(action.ConfigSnapshotJSON),
			AttemptCount:       action.AttemptCount,
			LastErrorCode:      action.LastErrorCode,
			LastError:          action.LastError,
			StartedAt:          action.StartedAt,
			FinishedAt:         action.FinishedAt,
			NextRetryAt:        action.NextRetryAt,
			Attempts:           attemptsByExecution[action.ID],
		})
	}
	return responses
}

func caseEventResponses(events []structs.CaseEvent) []CaseEventResponse {
	responses := make([]CaseEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, CaseEventResponse{
			ID:                 event.ID,
			CreatedAt:          event.CreatedAt,
			UpdatedAt:          event.UpdatedAt,
			EventType:          event.EventType,
			ActorDiscordUserID: event.ActorDiscordUserID,
			ActorType:          event.ActorType,
			Visibility:         event.Visibility,
			Body:               event.Body,
			Metadata:           parseJSON(event.MetadataJSON),
			EditedAt:           event.EditedAt,
		})
	}
	return responses
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

func templateSnapshotResponse(snapshotJSON string) *CaseTemplateSnapshotResponse {
	var snapshot CaseTemplateSnapshotResponse
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil || snapshot.Template.ID == "" {
		return nil
	}
	return &snapshot
}

func pagination(limitValue, offsetValue string) (int, int, error) {
	limit := 50
	if strings.TrimSpace(limitValue) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(limitValue))
		if err != nil || parsed <= 0 {
			return 0, 0, validationCaseError("limit must be a positive integer")
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
			return 0, 0, validationCaseError("offset must be a non-negative integer")
		}
		offset = parsed
	}

	return limit, offset, nil
}

func validCaseStatus(status structs.CaseStatus) bool {
	switch status {
	case structs.CaseStatusOpen, structs.CaseStatusActionRunning, structs.CaseStatusCompleted, structs.CaseStatusFailed, structs.CaseStatusAppealed, structs.CaseStatusVoided:
		return true
	default:
		return false
	}
}

func caseStatusSummary(source map[structs.CaseStatus]int64) map[string]int64 {
	out := make(map[string]int64, len(source))
	for status, count := range source {
		out[string(status)] = count
	}
	return out
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
	entry := s.auditEntry(ctx, guildContext, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return s.store.CreateAuditLogEntry(ctx, entry)
}

func (s *CaseService) auditEntry(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result structs.AuditResult, failureReason string) *structs.AuditLogEntry {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil
	}
	requestID, correlationID := TraceIDsFromContext(ctx)

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
		CorrelationID:       correlationID,
		RequestID:           requestID,
		MetadataJSON:        "{}",
	}
	if entry.ResourceID == "" {
		entry.ResourceID = "unknown"
	}
	return entry
}

func ensureTraceContext(ctx context.Context) context.Context {
	if RequestIDFromContext(ctx) != "" && CorrelationIDFromContext(ctx) != "" {
		return ctx
	}
	return ContextWithTrace(ctx, RequestIDFromContext(ctx), CorrelationIDFromContext(ctx))
}
