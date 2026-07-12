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
	ErrCaseValidation           = errors.New("case validation failed")
	ErrCaseTemplateNotAvailable = errors.New("case template not available")
	ErrCasePermissionDenied     = errors.New("case permission denied")
	ErrCaseNotFound             = errors.New("case not found")
)

// CaseService owns case authorization, escalation selection, snapshots, auditing, and action scheduling.
type CaseService struct {
	store     Repository
	scheduler CaseWorkScheduler
}

// CaseInput groups the validated inputs needed for case input.
type CaseInput struct {
	TemplateID              string           `json:"template_id"`
	TargetDiscordUserID     string           `json:"target_discord_user_id"`
	Source                  model.CaseSource `json:"source"`
	ContextChannelDiscordID string           `json:"context_channel_discord_id"`
	ContextMessageDiscordID string           `json:"context_message_discord_id"`
	ContextURL              string           `json:"context_url"`
	Metadata                json.RawMessage  `json:"metadata"`
}

// CaseListInput groups the validated inputs needed for case list input.
type CaseListInput struct {
	Limit                  string
	Offset                 string
	TargetDiscordUserID    string
	ModeratorDiscordUserID string
	TemplateID             string
	Validity               string
}

// CaseListResponse is the transport-neutral representation returned for case list response.
type CaseListResponse struct {
	Cases  []CaseResponse `json:"cases"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// CaseDetailResponse is the transport-neutral representation returned for case detail response.
type CaseDetailResponse struct {
	CaseResponse
	TemplateSnapshot *CaseTemplateSnapshotResponse `json:"template_snapshot,omitempty"`
	Actions          []CaseActionDetailResponse    `json:"actions"`
	Events           []CaseEventResponse           `json:"events"`
}

// CaseProfileResponse is the transport-neutral representation returned for case profile response.
type CaseProfileResponse struct {
	Cases   []CaseResponse     `json:"cases"`
	Total   int64              `json:"total"`
	Limit   int                `json:"limit"`
	Offset  int                `json:"offset"`
	Summary CaseProfileSummary `json:"summary"`
}

// CaseProfileSummary groups the case profile summary state used to keep this package's responsibilities explicit.
type CaseProfileSummary struct {
	Total      int64            `json:"total"`
	ByValidity map[string]int64 `json:"by_validity"`
	ByTemplate map[string]int64 `json:"by_template"`
}

// CaseResponse is the transport-neutral representation returned for case response.
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
	Validity                model.CaseValidity   `json:"validity"`
	Source                  model.CaseSource     `json:"source"`
	ContextChannelDiscordID string               `json:"context_channel_discord_id,omitempty"`
	ContextMessageDiscordID string               `json:"context_message_discord_id,omitempty"`
	ContextURL              string               `json:"context_url,omitempty"`
	Metadata                any                  `json:"metadata"`
	SelectedLevel           *CaseSelectedLevel   `json:"selected_level,omitempty"`
	Actions                 []CaseActionResponse `json:"actions"`
}

// CaseSelectedLevel groups the case selected level state used to keep this package's responsibilities explicit.
type CaseSelectedLevel struct {
	TemplateLevelDetails
	MatchedCaseCount int64 `json:"matched_case_count"`
}

// CaseActionResponse is the transport-neutral representation returned for case action response.
type CaseActionResponse struct {
	ID               string                      `json:"id"`
	Position         int                         `json:"position"`
	ActionType       model.ActionType            `json:"action_type"`
	Status           model.ActionExecutionStatus `json:"status"`
	TemplateActionID *string                     `json:"template_action_id"`
	IdempotencyKey   string                      `json:"idempotency_key"`
	NotifyUser       bool                        `json:"notify_user"`
	NotificationType string                      `json:"notification_type,omitempty"`
	MaxRetries       uint8                       `json:"max_retries"`
	RetryBackoffMS   int                         `json:"retry_backoff_ms"`
	SafeForRetry     bool                        `json:"safe_for_retry"`
	Irreversible     bool                        `json:"irreversible"`
}

// CaseActionDetailResponse is the transport-neutral representation returned for case action detail response.
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

// CaseActionAttemptResponse is the transport-neutral representation returned for case action attempt response.
type CaseActionAttemptResponse struct {
	ID              string                    `json:"id"`
	ExecutionID     string                    `json:"execution_id"`
	AttemptNumber   uint8                     `json:"attempt_number"`
	Status          model.ActionAttemptStatus `json:"status"`
	WorkerID        string                    `json:"worker_id,omitempty"`
	StartedAt       time.Time                 `json:"started_at"`
	FinishedAt      *time.Time                `json:"finished_at,omitempty"`
	DurationMS      int64                     `json:"duration_ms"`
	ErrorCode       string                    `json:"error_code,omitempty"`
	ErrorMessage    string                    `json:"error_message,omitempty"`
	RequestPayload  any                       `json:"request_payload"`
	ResponsePayload any                       `json:"response_payload"`
}

// CaseEventResponse is the transport-neutral representation returned for case event response.
type CaseEventResponse struct {
	ID                 string                `json:"id"`
	CreatedAt          time.Time             `json:"created_at"`
	UpdatedAt          time.Time             `json:"updated_at"`
	EventType          model.CaseEventType   `json:"event_type"`
	ActorDiscordUserID string                `json:"actor_discord_user_id,omitempty"`
	ActorType          string                `json:"actor_type"`
	Visibility         model.EventVisibility `json:"visibility"`
	Body               string                `json:"body"`
	Metadata           any                   `json:"metadata"`
}

// CaseTemplateSnapshotResponse is the transport-neutral representation returned for case template snapshot response.
type CaseTemplateSnapshotResponse struct {
	Template      templateSnapshotTemplate `json:"template"`
	SelectedLevel CaseSelectedLevel        `json:"selected_level"`
	Actions       []templateSnapshotAction `json:"actions"`
}

// templateSnapshot groups the template snapshot state used to keep this package's responsibilities explicit.
type templateSnapshot struct {
	Template      templateSnapshotTemplate `json:"template"`
	SelectedLevel CaseSelectedLevel        `json:"selected_level"`
	Actions       []templateSnapshotAction `json:"actions"`
}

// templateSnapshotTemplate groups the template snapshot template state used to keep this package's responsibilities explicit.
type templateSnapshotTemplate struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Version        uint   `json:"version"`
	ReasonTemplate string `json:"reason_template"`
}

// templateSnapshotAction identifies the supported template snapshot action values stored and exchanged by Quack.
type templateSnapshotAction struct {
	ID                     string           `json:"id"`
	ActionType             model.ActionType `json:"action_type"`
	TimeoutDurationSeconds int              `json:"timeout_duration_seconds,omitempty"`
	DeleteMessageSeconds   int              `json:"delete_message_seconds,omitempty"`
	MaxRetries             uint8            `json:"max_retries"`
}

// selectedTemplateLevel groups the selected template level state used to keep this package's responsibilities explicit.
type selectedTemplateLevel struct {
	Level            model.CaseTemplateLevel
	Actions          []model.CaseTemplateLevelAction
	MatchedCaseCount int64
}

// NewCaseService constructs case service with required dependencies explicit so callers control lifecycle and substitution.
func NewCaseService(store Repository, scheduler ...CaseWorkScheduler) *CaseService {
	service := &CaseService{store: store}
	if len(scheduler) > 0 {
		service.scheduler = scheduler[0]
	}
	return service
}

// Create applies a template to a user inside the guild-scoped transaction boundary. The lock keeps escalation history and case numbering consistent, while scheduling occurs only after the transaction commits.
func (s *CaseService) Create(ctx context.Context, guildContext *GuildStaffContext, input CaseInput) (*CaseResponse, error) {
	ctx = ensureTraceContext(ctx)
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil {
		return nil, validationCaseError("missing guild context")
	}
	var created *model.CreatedCase
	err := s.store.WithGuildCaseLock(ctx, guildContext.Guild.ID, func(transactionalStore Repository) error {
		transactionalService := *s
		transactionalService.store = transactionalStore
		var createErr error
		created, createErr = transactionalService.create(ctx, guildContext, input)
		return createErr
	})
	if err != nil {
		if errors.Is(err, ErrCaseValidation) || errors.Is(err, ErrCasePermissionDenied) || errors.Is(err, ErrCaseTemplateNotAvailable) {
			_ = s.audit(ctx, guildContext, "case.create", "case", "unknown", model.AuditResultFailure, err.Error())
		}
		return nil, err
	}

	if s.scheduler != nil {
		s.scheduler.Submit(ctx, created.Case.ID)
	}

	response := caseResponse(*created)
	return &response, nil
}

// List returns list subject to authorization, ordering, and filtering constraints.
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

// Get retrieves get without exposing the underlying adapter implementation.
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

// UserHistory encapsulates the user history rule so callers share one consistent package implementation.
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
			ByValidity: caseValiditySummary(summary.ByValidity),
			ByTemplate: summary.ByTemplate,
		},
	}, nil
}

// caseListParams encapsulates the case list params rule so callers share one consistent package implementation.
func (s *CaseService) caseListParams(guildContext *GuildStaffContext, input CaseListInput) (model.ListCasesParams, int, int, error) {
	if err := s.requireCaseRead(guildContext); err != nil {
		return model.ListCasesParams{}, 0, 0, err
	}

	limit, offset, err := pagination(input.Limit, input.Offset)
	if err != nil {
		return model.ListCasesParams{}, 0, 0, err
	}

	validity := model.CaseValidity(strings.TrimSpace(input.Validity))
	if validity != "" && !validCaseValidity(validity) {
		return model.ListCasesParams{}, 0, 0, validationCaseError("validity is invalid")
	}

	return model.ListCasesParams{
		GuildID:                guildContext.Guild.ID,
		TargetDiscordUserID:    strings.TrimSpace(input.TargetDiscordUserID),
		ModeratorDiscordUserID: strings.TrimSpace(input.ModeratorDiscordUserID),
		TemplateID:             strings.TrimSpace(input.TemplateID),
		Validity:               validity,
		Limit:                  limit,
		Offset:                 offset,
	}, limit, offset, nil
}

// requireCaseRead encapsulates the require case read rule so callers share one consistent package implementation.
func (s *CaseService) requireCaseRead(guildContext *GuildStaffContext) error {
	if s == nil || s.store == nil {
		return errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return validationCaseError("missing guild context")
	}
	if !guildContext.Can(model.PermissionActionCaseCreate) {
		return ErrCasePermissionDenied
	}
	return nil
}

// create validates and materializes a case within an already locked transaction, including the selected escalation level, immutable template snapshot, initial event, actions, and audit entry.
func (s *CaseService) create(ctx context.Context, guildContext *GuildStaffContext, input CaseInput) (*model.CreatedCase, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, validationCaseError("missing guild context")
	}
	if !guildContext.Can(model.PermissionActionCaseCreate) {
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
		source = model.CaseSourceDashboard
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
	if template == nil || template.Template.ArchivedAt != nil {
		return nil, ErrCaseTemplateNotAvailable
	}

	reason := strings.TrimSpace(template.Template.ReasonTemplate)
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

	caseModel := model.Case{
		GuildID:                 guildContext.Guild.ID,
		TemplateID:              &template.Template.ID,
		TemplateVersion:         template.Template.Version,
		TemplateSnapshotJSON:    snapshotJSON,
		TargetDiscordUserID:     targetDiscordUserID,
		ModeratorDiscordUserID:  guildContext.Staff.DiscordUserID,
		Reason:                  reason,
		Validity:                model.CaseValidityValid,
		Source:                  source,
		CorrelationID:           correlationID,
		ContextChannelDiscordID: strings.TrimSpace(input.ContextChannelDiscordID),
		ContextMessageDiscordID: strings.TrimSpace(input.ContextMessageDiscordID),
		ContextURL:              strings.TrimSpace(input.ContextURL),
		MetadataJSON:            metadataJSON,
	}

	actionExecutions := make([]model.CaseActionExecution, 0, len(selectedLevel.Actions)+1)
	if selectedLevel.Level.NotifyUser {
		actionExecutions = append(actionExecutions, warningNotificationExecution(caseModel))
	}
	for _, action := range selectedLevel.Actions {
		templateActionID := action.ID
		actionExecutions = append(actionExecutions, model.CaseActionExecution{
			TemplateActionID:   &templateActionID,
			Position:           1,
			ActionType:         action.ActionType,
			Status:             model.ActionExecutionPending,
			ConfigSnapshotJSON: action.ConfigJSON,
			MaxRetries:         action.MaxRetries,
			RetryBackoffMS:     1000,
			SafeForRetry:       !irreversibleAction(action.ActionType),
			Irreversible:       irreversibleAction(action.ActionType),
			CorrelationID:      correlationID,
		})
	}

	event := model.CaseEvent{
		EventType:          model.CaseEventCreated,
		ActorDiscordUserID: guildContext.Staff.DiscordUserID,
		ActorType:          "staff",
		Visibility:         model.EventVisibilityStaff,
		Body:               fmt.Sprintf("Case created from template %s", template.Template.Slug),
		MetadataJSON:       "{}",
	}

	return s.store.CreateCase(ctx, model.CreateCaseParams{
		Case:             caseModel,
		Event:            event,
		ActionExecutions: actionExecutions,
		Audit:            s.auditEntry(ctx, guildContext, "case.create", "case", "", model.AuditResultSuccess, ""),
	})
}

// selectTemplateLevel chooses the highest escalation whose all-time historical-case threshold is met, falling back to the default level.
func (s *CaseService) selectTemplateLevel(ctx context.Context, guildID, targetDiscordUserID string, template *model.ExpandedCaseTemplate) (*selectedTemplateLevel, error) {
	if template == nil {
		return nil, validationCaseError("template is required")
	}

	var fallback *selectedTemplateLevel
	var best *selectedTemplateLevel
	for _, expandedLevel := range template.Levels {
		level := expandedLevel.Level
		if len(expandedLevel.Actions) > 1 {
			return nil, validationCaseError("template level has more than one enforcement action")
		}
		if level.IsDefault {
			matchedCaseCount, err := s.matchingTemplateCaseCount(ctx, guildID, targetDiscordUserID, template.Template.ID)
			if err != nil {
				return nil, err
			}
			fallback = &selectedTemplateLevel{
				Level:            level,
				Actions:          expandedLevel.Actions,
				MatchedCaseCount: matchedCaseCount,
			}
			continue
		}

		if level.TriggerCaseCount <= 0 {
			return nil, validationCaseError("escalation level trigger_case_count must be positive")
		}

		matchedCaseCount, err := s.matchingTemplateCaseCount(ctx, guildID, targetDiscordUserID, template.Template.ID)
		if err != nil {
			return nil, err
		}
		if matchedCaseCount < int64(level.TriggerCaseCount) {
			continue
		}
		candidate := &selectedTemplateLevel{
			Level:            level,
			Actions:          expandedLevel.Actions,
			MatchedCaseCount: matchedCaseCount,
		}
		if best == nil || level.TriggerCaseCount > best.Level.TriggerCaseCount {
			best = candidate
		}
	}

	if fallback == nil {
		return nil, validationCaseError("template has no default level")
	}
	if best != nil {
		return best, nil
	}

	return fallback, nil
}

// matchingTemplateCaseCount returns the relevant historical count plus the case currently being created, matching the user-facing meaning of a trigger count.
func (s *CaseService) matchingTemplateCaseCount(ctx context.Context, guildID, targetDiscordUserID, templateID string) (int64, error) {
	priorCount, err := s.store.CountTemplateCasesForTarget(ctx, model.CountTemplateCasesForTargetParams{
		GuildID:             guildID,
		TemplateID:          templateID,
		TargetDiscordUserID: targetDiscordUserID,
	})
	if err != nil {
		return 0, err
	}
	return priorCount + 1, nil
}

// buildTemplateSnapshot builds template snapshot from validated domain state.
func buildTemplateSnapshot(template model.CaseTemplate, selectedLevel selectedTemplateLevel) (string, error) {
	snapshot := templateSnapshot{
		Template: templateSnapshotTemplate{
			ID:             template.ID,
			Slug:           template.Slug,
			Name:           template.Name,
			Version:        template.Version,
			ReasonTemplate: template.ReasonTemplate,
		},
		SelectedLevel: CaseSelectedLevel{
			TemplateLevelDetails: templateLevelDetails(selectedLevel.Level),
			MatchedCaseCount:     selectedLevel.MatchedCaseCount,
		},
		Actions: make([]templateSnapshotAction, 0, len(selectedLevel.Actions)),
	}

	for _, action := range selectedLevel.Actions {
		settings := templateActionResponse(action)
		snapshot.Actions = append(snapshot.Actions, templateSnapshotAction{
			ID:                     action.ID,
			ActionType:             action.ActionType,
			TimeoutDurationSeconds: settings.TimeoutDurationSeconds,
			DeleteMessageSeconds:   settings.DeleteMessageSeconds,
			MaxRetries:             action.MaxRetries,
		})
	}

	body, err := json.Marshal(snapshot)
	if err != nil {
		return "", fmt.Errorf("marshal case template snapshot: %w", err)
	}
	return string(body), nil
}

// warningNotificationExecution encapsulates the warning notification execution rule so callers share one consistent package implementation.
func warningNotificationExecution(caseModel model.Case) model.CaseActionExecution {
	return model.CaseActionExecution{
		Position:           0,
		ActionType:         model.ActionSendDM,
		Status:             model.ActionExecutionPending,
		ConfigSnapshotJSON: warningNotificationConfig(caseModel.Reason),
		NotificationType:   string(model.NotificationWarning),
		SafeForRetry:       true,
		CorrelationID:      caseModel.CorrelationID,
	}
}

// warningNotificationConfig encapsulates the warning notification config rule so callers share one consistent package implementation.
func warningNotificationConfig(reason string) string {
	body, err := json.Marshal(map[string]any{
		"message": fmt.Sprintf("You received a warning in this server: %s", reason),
	})
	if err != nil {
		return "{}"
	}
	return string(body)
}

// caseResponse converts case response into its transport presentation without leaking transport types into the core.
func caseResponse(created model.CreatedCase) CaseResponse {
	return caseResponseFromModel(created.Case, created.ActionExecutions)
}

// caseResponseFromModel encapsulates the case response from model rule so callers share one consistent package implementation.
func caseResponseFromModel(caseModel model.Case, actionExecutions []model.CaseActionExecution) CaseResponse {
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
		Validity:                caseModel.Validity,
		Source:                  caseModel.Source,
		ContextChannelDiscordID: caseModel.ContextChannelDiscordID,
		ContextMessageDiscordID: caseModel.ContextMessageDiscordID,
		ContextURL:              caseModel.ContextURL,
		Metadata:                parseJSON(caseModel.MetadataJSON),
		SelectedLevel:           selectedLevelResponse(caseModel.TemplateSnapshotJSON),
		Actions:                 make([]CaseActionResponse, 0, len(actionExecutions)),
	}

	for _, action := range actionExecutions {
		response.Actions = append(response.Actions, caseActionResponse(action))
	}

	return response
}

// caseResponsesForModels encapsulates the case responses for models rule so callers share one consistent package implementation.
func (s *CaseService) caseResponsesForModels(ctx context.Context, cases []model.Case) ([]CaseResponse, error) {
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

// caseActionResponse converts case action response into its transport presentation without leaking transport types into the core.
func caseActionResponse(action model.CaseActionExecution) CaseActionResponse {
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

// caseActionDetailResponses converts case action detail responses into its transport presentation without leaking transport types into the core.
func caseActionDetailResponses(actions []model.CaseActionExecution, attempts []model.CaseActionAttempt) []CaseActionDetailResponse {
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

// caseEventResponses converts case event responses into its transport presentation without leaking transport types into the core.
func caseEventResponses(events []model.CaseEvent) []CaseEventResponse {
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
		})
	}
	return responses
}

// selectedLevelResponse converts selected level response into its transport presentation without leaking transport types into the core.
func selectedLevelResponse(snapshotJSON string) *CaseSelectedLevel {
	var snapshot struct {
		SelectedLevel CaseSelectedLevel `json:"selected_level"`
	}
	if err := json.Unmarshal([]byte(snapshotJSON), &snapshot); err != nil || snapshot.SelectedLevel.ID == "" {
		return nil
	}
	return &snapshot.SelectedLevel
}

// templateSnapshotResponse converts template snapshot response into its transport presentation without leaking transport types into the core.
func templateSnapshotResponse(snapshotJSON string) *CaseTemplateSnapshotResponse {
	var stored struct {
		Template      templateSnapshotTemplate `json:"template"`
		SelectedLevel CaseSelectedLevel        `json:"selected_level"`
		Actions       []json.RawMessage        `json:"actions"`
	}
	if err := json.Unmarshal([]byte(snapshotJSON), &stored); err != nil || stored.Template.ID == "" {
		return nil
	}
	snapshot := CaseTemplateSnapshotResponse{
		Template:      stored.Template,
		SelectedLevel: stored.SelectedLevel,
		Actions:       make([]templateSnapshotAction, 0, len(stored.Actions)),
	}
	for _, raw := range stored.Actions {
		var action templateSnapshotAction
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		if action.TimeoutDurationSeconds == 0 && action.DeleteMessageSeconds == 0 {
			var legacy struct {
				Config json.RawMessage `json:"config"`
			}
			if err := json.Unmarshal(raw, &legacy); err == nil && len(legacy.Config) > 0 {
				settings := decodeTemplateActionConfig(string(legacy.Config))
				action.TimeoutDurationSeconds = settings.DurationSeconds
				action.DeleteMessageSeconds = settings.DeleteMessageSeconds
			}
		}
		snapshot.Actions = append(snapshot.Actions, action)
	}
	return &snapshot
}

// pagination encapsulates the pagination rule so callers share one consistent package implementation.
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

// validCaseValidity reports whether validity is one of the two v5 case states.
func validCaseValidity(validity model.CaseValidity) bool {
	switch validity {
	case model.CaseValidityValid, model.CaseValidityVoided:
		return true
	default:
		return false
	}
}

// caseValiditySummary converts typed validity counts for transport responses.
func caseValiditySummary(source map[model.CaseValidity]int64) map[string]int64 {
	out := make(map[string]int64, len(source))
	for status, count := range source {
		out[string(status)] = count
	}
	return out
}

// validationCaseError checks validation case error before state is read or changed.
func validationCaseError(message string) error {
	return fmt.Errorf("%w: %s", ErrCaseValidation, message)
}

// validCaseSource checks valid case source before state is read or changed.
func validCaseSource(source model.CaseSource) bool {
	switch source {
	case model.CaseSourceDashboard, model.CaseSourceDiscord, model.CaseSourceHoneypot, model.CaseSourceV4Import:
		return true
	default:
		return false
	}
}

// irreversibleAction encapsulates the irreversible action rule so callers share one consistent package implementation.
func irreversibleAction(actionType model.ActionType) bool {
	switch actionType {
	case model.ActionTimeoutUser, model.ActionKickUser, model.ActionBanUser:
		return true
	default:
		return false
	}
}

// audit records audit so moderation changes remain attributable.
func (s *CaseService) audit(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) error {
	entry := s.auditEntry(ctx, guildContext, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return s.store.CreateAuditLogEntry(ctx, entry)
}

// auditEntry records audit entry so moderation changes remain attributable.
func (s *CaseService) auditEntry(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) *model.AuditLogEntry {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil
	}
	requestID, correlationID := TraceIDsFromContext(ctx)

	entry := &model.AuditLogEntry{
		GuildID:             guildContext.Guild.ID,
		ActorDiscordUserID:  guildContext.Staff.DiscordUserID,
		ActorPermissionBits: guildContext.PermissionBits,
		Source:              model.AuditSourceAPI,
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

// ensureTraceContext encapsulates the ensure trace context rule so callers share one consistent package implementation.
func ensureTraceContext(ctx context.Context) context.Context {
	if RequestIDFromContext(ctx) != "" && CorrelationIDFromContext(ctx) != "" {
		return ctx
	}
	return ContextWithTrace(ctx, RequestIDFromContext(ctx), CorrelationIDFromContext(ctx))
}
