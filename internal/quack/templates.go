package quack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/quackdiscord/bot/internal/quack/model"
)

var (
	ErrTemplateValidation = errors.New("template validation failed")
	ErrTemplateNotFound   = errors.New("case template not found")
)

var templateSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// TemplateService owns template validation, normalization, persistence, and audit creation.
type TemplateService struct {
	store Repository
}

// TemplateInput groups the validated inputs needed for template input.
type TemplateInput struct {
	Slug           string               `json:"slug"`
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	ReasonTemplate string               `json:"reason_template"`
	Appealable     bool                 `json:"appealable"`
	Enabled        *bool                `json:"enabled"`
	Levels         []TemplateLevelInput `json:"levels"`
}

// TemplateLevelInput groups the validated inputs needed for template level input.
type TemplateLevelInput struct {
	Name             string                `json:"name"`
	Position         int                   `json:"position"`
	IsDefault        bool                  `json:"is_default"`
	TriggerCaseCount int                   `json:"trigger_case_count"`
	WindowMinutes    int                   `json:"window_minutes"`
	NotifyUser       bool                  `json:"notify_user"`
	NotificationType string                `json:"notification_type"`
	Enabled          *bool                 `json:"enabled"`
	Actions          []TemplateActionInput `json:"actions"`
}

// TemplateActionInput groups the validated inputs needed for template action input.
type TemplateActionInput struct {
	ActionType       model.ActionType `json:"action_type"`
	Config           json.RawMessage  `json:"config"`
	NotifyUser       bool             `json:"notify_user"`
	NotificationType string           `json:"notification_type"`
	ContinueOnError  bool             `json:"continue_on_error"`
	MaxRetries       int              `json:"max_retries"`
	RetryBackoffMS   int              `json:"retry_backoff_ms"`
	TimeoutMS        int              `json:"timeout_ms"`
	IdempotencyScope string           `json:"idempotency_scope"`
	Enabled          *bool            `json:"enabled"`
}

// TemplateResponse is the transport-neutral representation returned for template response.
type TemplateResponse struct {
	ID                     string                  `json:"id"`
	GuildID                string                  `json:"guild_id"`
	Slug                   string                  `json:"slug"`
	Name                   string                  `json:"name"`
	Description            string                  `json:"description"`
	ReasonTemplate         string                  `json:"reason_template"`
	Appealable             bool                    `json:"appealable"`
	Enabled                bool                    `json:"enabled"`
	Version                uint                    `json:"version"`
	CreatedByDiscordUserID string                  `json:"created_by_discord_user_id"`
	UpdatedByDiscordUserID string                  `json:"updated_by_discord_user_id"`
	ArchivedAt             any                     `json:"archived_at"`
	Levels                 []TemplateLevelResponse `json:"levels"`
}

// TemplateLevelDetails groups the template level details state used to keep this package's responsibilities explicit.
type TemplateLevelDetails struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Position         int    `json:"position"`
	IsDefault        bool   `json:"is_default"`
	TriggerCaseCount int    `json:"trigger_case_count"`
	WindowMinutes    int    `json:"window_minutes"`
	NotifyUser       bool   `json:"notify_user"`
	NotificationType string `json:"notification_type,omitempty"`
}

// TemplateLevelResponse is the transport-neutral representation returned for template level response.
type TemplateLevelResponse struct {
	TemplateLevelDetails
	Enabled bool                     `json:"enabled"`
	Actions []TemplateActionResponse `json:"actions"`
}

// TemplateActionResponse is the transport-neutral representation returned for template action response.
type TemplateActionResponse struct {
	ID               string           `json:"id"`
	Position         int              `json:"position"`
	ActionType       model.ActionType `json:"action_type"`
	Config           any              `json:"config"`
	NotifyUser       bool             `json:"notify_user"`
	NotificationType string           `json:"notification_type,omitempty"`
	ContinueOnError  bool             `json:"continue_on_error"`
	MaxRetries       uint8            `json:"max_retries"`
	RetryBackoffMS   int              `json:"retry_backoff_ms"`
	TimeoutMS        int              `json:"timeout_ms"`
	IdempotencyScope string           `json:"idempotency_scope"`
	Enabled          bool             `json:"enabled"`
}

// NewTemplateService constructs template service with required dependencies explicit so callers control lifecycle and substitution.
func NewTemplateService(store Repository) *TemplateService {
	return &TemplateService{store: store}
}

// List returns list subject to authorization, ordering, and filtering constraints.
func (s *TemplateService) List(ctx context.Context, guildContext *GuildStaffContext) ([]TemplateResponse, error) {
	templates, err := s.store.ListCaseTemplates(ctx, guildContext.Guild.ID)
	if err != nil {
		return nil, err
	}

	out := make([]TemplateResponse, 0, len(templates))
	for _, template := range templates {
		out = append(out, templateResponse(template))
	}
	return out, nil
}

// Get retrieves get without exposing the underlying adapter implementation.
func (s *TemplateService) Get(ctx context.Context, guildContext *GuildStaffContext, templateID string) (*TemplateResponse, error) {
	template, err := s.store.GetCaseTemplateExpanded(ctx, guildContext.Guild.ID, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, ErrTemplateNotFound
	}

	response := templateResponse(*template)
	return &response, nil
}

// Create validates, normalizes, and persists a new guild template with its escalation levels and actions, then records the moderation audit entry.
func (s *TemplateService) Create(ctx context.Context, guildContext *GuildStaffContext, input TemplateInput) (*TemplateResponse, error) {
	ctx = ensureTraceContext(ctx)
	normalized, err := s.validate(ctx, guildContext, "", input)
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.create", "case_template", "unknown", model.AuditResultFailure, err.Error())
		return nil, err
	}

	expanded, err := s.store.CreateCaseTemplate(ctx, model.CreateCaseTemplateParams{
		Template: normalized.template,
		Levels:   normalized.levels,
		Audit:    s.auditEntry(ctx, guildContext, "case_template.create", "case_template", "", model.AuditResultSuccess, ""),
	})
	if err != nil {
		return nil, err
	}

	response := templateResponse(*expanded)
	return &response, nil
}

// Update updates update while retaining validation, compatibility, and audit requirements.
func (s *TemplateService) Update(ctx context.Context, guildContext *GuildStaffContext, templateID string, input TemplateInput) (*TemplateResponse, error) {
	ctx = ensureTraceContext(ctx)
	existing, err := s.store.GetCaseTemplateExpanded(ctx, guildContext.Guild.ID, templateID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTemplateNotFound
	}

	normalized, err := s.validate(ctx, guildContext, templateID, input)
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.update", "case_template", templateID, model.AuditResultFailure, err.Error())
		return nil, err
	}

	expanded, err := s.store.UpdateCaseTemplate(ctx, model.UpdateCaseTemplateParams{
		GuildID:    guildContext.Guild.ID,
		TemplateID: templateID,
		Template:   normalized.template,
		Levels:     normalized.levels,
		Audit:      s.auditEntry(ctx, guildContext, "case_template.update", "case_template", templateID, model.AuditResultSuccess, ""),
	})
	if err != nil {
		return nil, err
	}
	if expanded == nil {
		return nil, ErrTemplateNotFound
	}

	response := templateResponse(*expanded)
	return &response, nil
}

// Archive archives archive without deleting historical moderation references.
func (s *TemplateService) Archive(ctx context.Context, guildContext *GuildStaffContext, templateID string) (*TemplateResponse, error) {
	ctx = ensureTraceContext(ctx)
	expanded, err := s.store.ArchiveCaseTemplate(
		ctx,
		guildContext.Guild.ID,
		templateID,
		s.auditEntry(ctx, guildContext, "case_template.archive", "case_template", templateID, model.AuditResultSuccess, ""),
	)
	if err != nil {
		return nil, err
	}
	if expanded == nil {
		return nil, ErrTemplateNotFound
	}

	response := templateResponse(*expanded)
	return &response, nil
}

// normalizedTemplate groups the normalized template state used to keep this package's responsibilities explicit.
type normalizedTemplate struct {
	template model.CaseTemplate
	levels   []model.ExpandedCaseTemplateLevel
}

// validate checks validate before state is read or changed.
func (s *TemplateService) validate(ctx context.Context, guildContext *GuildStaffContext, templateID string, input TemplateInput) (*normalizedTemplate, error) {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, validationError("missing guild context")
	}

	slug := strings.ToLower(strings.TrimSpace(input.Slug))
	if !templateSlugPattern.MatchString(slug) {
		return nil, validationError("slug must be 2-64 lowercase letters, numbers, underscores, or hyphens")
	}

	if existing, err := s.store.GetCaseTemplateBySlug(ctx, guildContext.Guild.ID, slug); err != nil {
		return nil, err
	} else if existing != nil && existing.ID != templateID {
		return nil, validationError("slug already exists for this guild")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, validationError("name is required")
	}

	reasonTemplate := strings.TrimSpace(input.ReasonTemplate)
	if reasonTemplate == "" {
		return nil, validationError("reason_template is required")
	}

	levels, err := normalizeLevels(input.Levels)
	if err != nil {
		return nil, err
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	template := model.CaseTemplate{
		GuildID:                guildContext.Guild.ID,
		Slug:                   slug,
		Name:                   name,
		Description:            strings.TrimSpace(input.Description),
		ReasonTemplate:         reasonTemplate,
		DefaultSeverity:        model.CaseSeverityMedium,
		Appealable:             input.Appealable,
		Enabled:                enabled,
		CreatedByDiscordUserID: guildContext.Staff.DiscordUserID,
		UpdatedByDiscordUserID: guildContext.Staff.DiscordUserID,
	}

	return &normalizedTemplate{template: template, levels: levels}, nil
}

// normalizeLevels produces a stable levels representation for deterministic validation, comparison, or caching.
func normalizeLevels(inputs []TemplateLevelInput) ([]model.ExpandedCaseTemplateLevel, error) {
	if len(inputs) == 0 {
		return nil, validationError("at least one level is required")
	}

	levels := make([]model.ExpandedCaseTemplateLevel, 0, len(inputs))
	defaultCount := 0

	for i, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, validationError("level name is required")
		}
		if input.TriggerCaseCount < 0 || input.WindowMinutes < 0 {
			return nil, validationError("level trigger values must be non-negative")
		}
		if !input.IsDefault && input.TriggerCaseCount <= 0 {
			return nil, validationError("escalation level trigger_case_count must be positive")
		}
		if input.IsDefault {
			defaultCount++
			if input.TriggerCaseCount != 0 || input.WindowMinutes != 0 {
				return nil, validationError("default level cannot have escalation triggers")
			}
		}
		notificationType, err := normalizeLevelNotificationType(input.NotifyUser, input.NotificationType)
		if err != nil {
			return nil, err
		}

		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		actions, _, err := normalizeActions(input.Actions)
		if err != nil {
			return nil, err
		}

		position := input.Position
		if position == 0 {
			position = i + 1
		}
		if position < 0 {
			return nil, validationError("level position must be non-negative")
		}

		levels = append(levels, model.ExpandedCaseTemplateLevel{
			Level: model.CaseTemplateLevel{
				Position:         position,
				Name:             name,
				IsDefault:        input.IsDefault,
				TriggerCaseCount: input.TriggerCaseCount,
				WindowMinutes:    input.WindowMinutes,
				NotifyUser:       input.NotifyUser,
				NotificationType: notificationType,
				Enabled:          enabled,
			},
			Actions: actions,
		})
	}

	if defaultCount == 0 {
		return nil, validationError("exactly one default level is required")
	}
	if defaultCount > 1 {
		return nil, validationError("only one default level is allowed")
	}

	return levels, nil
}

// normalizeActions produces a stable actions representation for deterministic validation, comparison, or caching.
func normalizeActions(inputs []TemplateActionInput) ([]model.CaseTemplateLevelAction, int, error) {
	actions := make([]model.CaseTemplateLevelAction, 0, len(inputs))
	enabledCount := 0
	for i, input := range inputs {
		if input.ActionType == "record_warning" {
			return nil, 0, validationError("record_warning is not a template action; creating a case records the warning")
		}
		if input.ActionType == model.ActionSendDM {
			return nil, 0, validationError("send_dm is not a template action; set notify_user on the level or moderation action")
		}
		if !validActionType(input.ActionType) {
			return nil, 0, validationError("action_type is invalid")
		}
		if input.MaxRetries < 0 || input.MaxRetries > 255 {
			return nil, 0, validationError("max_retries must be between 0 and 255")
		}
		if input.RetryBackoffMS < 0 {
			return nil, 0, validationError("retry_backoff_ms must be non-negative")
		}
		if input.TimeoutMS < 0 {
			return nil, 0, validationError("timeout_ms must be non-negative")
		}

		configJSON, err := normalizeJSONObject(input.Config)
		if err != nil {
			return nil, 0, validationError("config must be a JSON object")
		}
		notificationType, err := normalizeNotificationType(input.ActionType, input.NotifyUser, input.NotificationType)
		if err != nil {
			return nil, 0, err
		}

		idempotencyScope := strings.TrimSpace(input.IdempotencyScope)
		if idempotencyScope == "" {
			idempotencyScope = "case"
		}
		if len(idempotencyScope) > 32 {
			return nil, 0, validationError("idempotency_scope must be 32 characters or fewer")
		}

		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		if enabled {
			enabledCount++
		}

		actions = append(actions, model.CaseTemplateLevelAction{
			Position:         i + 1,
			ActionType:       input.ActionType,
			ConfigJSON:       configJSON,
			NotifyUser:       input.NotifyUser,
			NotificationType: notificationType,
			ContinueOnError:  input.ContinueOnError,
			MaxRetries:       uint8(input.MaxRetries),
			RetryBackoffMS:   input.RetryBackoffMS,
			TimeoutMS:        input.TimeoutMS,
			IdempotencyScope: idempotencyScope,
			Enabled:          enabled,
		})
	}

	return actions, enabledCount, nil
}

// normalizeJSONObject produces a stable jsonobject representation for deterministic validation, comparison, or caching.
func normalizeJSONObject(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "{}", nil
	}

	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return "", err
	}
	if object == nil {
		return "", errors.New("not an object")
	}

	body, err := json.Marshal(object)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// validActionType checks valid action type before state is read or changed.
func validActionType(actionType model.ActionType) bool {
	switch actionType {
	case model.ActionTimeoutUser, model.ActionKickUser, model.ActionBanUser:
		return true
	default:
		return false
	}
}

// normalizeLevelNotificationType produces a stable level notification type representation for deterministic validation, comparison, or caching.
func normalizeLevelNotificationType(notifyUser bool, notificationType string) (string, error) {
	if !notifyUser {
		return "", nil
	}
	normalized := strings.ToLower(strings.TrimSpace(notificationType))
	if normalized == "" {
		normalized = string(model.NotificationWarning)
	}
	if len(normalized) > 64 {
		return "", validationError("notification_type must be 64 characters or fewer")
	}
	if !validNotificationType(normalized) {
		return "", validationError("notification_type is invalid")
	}
	return normalized, nil
}

// normalizeNotificationType produces a stable notification type representation for deterministic validation, comparison, or caching.
func normalizeNotificationType(actionType model.ActionType, notifyUser bool, notificationType string) (string, error) {
	if !notifyUser {
		return "", nil
	}

	normalized := strings.ToLower(strings.TrimSpace(notificationType))
	if normalized == "" {
		normalized = defaultNotificationType(actionType)
	}
	if len(normalized) > 64 {
		return "", validationError("notification_type must be 64 characters or fewer")
	}
	if !validNotificationType(normalized) {
		return "", validationError("notification_type is invalid")
	}
	return normalized, nil
}

// defaultNotificationType encapsulates the default notification type rule so callers share one consistent package implementation.
func defaultNotificationType(actionType model.ActionType) string {
	switch actionType {
	case model.ActionTimeoutUser:
		return string(model.NotificationTimeout)
	case model.ActionKickUser:
		return string(model.NotificationKick)
	case model.ActionBanUser:
		return string(model.NotificationBan)
	default:
		return ""
	}
}

// validNotificationType checks valid notification type before state is read or changed.
func validNotificationType(notificationType string) bool {
	switch model.NotificationType(notificationType) {
	case model.NotificationWarning, model.NotificationTimeout, model.NotificationKick, model.NotificationBan:
		return true
	default:
		return false
	}
}

// validationError checks validation error before state is read or changed.
func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrTemplateValidation, message)
}

// audit records audit so moderation changes remain attributable.
func (s *TemplateService) audit(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) error {
	entry := s.auditEntry(ctx, guildContext, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return s.store.CreateAuditLogEntry(ctx, entry)
}

// auditEntry records audit entry so moderation changes remain attributable.
func (s *TemplateService) auditEntry(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) *model.AuditLogEntry {
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

// templateResponse converts template response into its transport presentation without leaking transport types into the core.
func templateResponse(expanded model.ExpandedCaseTemplate) TemplateResponse {
	template := expanded.Template
	response := TemplateResponse{
		ID:                     template.ID,
		GuildID:                template.GuildID,
		Slug:                   template.Slug,
		Name:                   template.Name,
		Description:            template.Description,
		ReasonTemplate:         template.ReasonTemplate,
		Appealable:             template.Appealable,
		Enabled:                template.Enabled,
		Version:                template.Version,
		CreatedByDiscordUserID: template.CreatedByDiscordUserID,
		UpdatedByDiscordUserID: template.UpdatedByDiscordUserID,
		ArchivedAt:             template.ArchivedAt,
		Levels:                 make([]TemplateLevelResponse, 0, len(expanded.Levels)),
	}

	for _, level := range expanded.Levels {
		levelResponse := TemplateLevelResponse{
			TemplateLevelDetails: templateLevelDetails(level.Level),
			Enabled:              level.Level.Enabled,
			Actions:              make([]TemplateActionResponse, 0, len(level.Actions)),
		}
		for _, action := range level.Actions {
			levelResponse.Actions = append(levelResponse.Actions, TemplateActionResponse{
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
				Enabled:          action.Enabled,
			})
		}
		response.Levels = append(response.Levels, levelResponse)
	}

	return response
}

// templateLevelDetails encapsulates the template level details rule so callers share one consistent package implementation.
func templateLevelDetails(level model.CaseTemplateLevel) TemplateLevelDetails {
	return TemplateLevelDetails{
		ID:               level.ID,
		Name:             level.Name,
		Position:         level.Position,
		IsDefault:        level.IsDefault,
		TriggerCaseCount: level.TriggerCaseCount,
		WindowMinutes:    level.WindowMinutes,
		NotifyUser:       level.NotifyUser,
		NotificationType: level.NotificationType,
	}
}

// enabledLevelActions encapsulates the enabled level actions rule so callers share one consistent package implementation.
func enabledLevelActions(actions []model.CaseTemplateLevelAction) []model.CaseTemplateLevelAction {
	enabled := make([]model.CaseTemplateLevelAction, 0, len(actions))
	for _, action := range actions {
		if action.Enabled {
			enabled = append(enabled, action)
		}
	}
	return enabled
}

// levelActionResponses converts level action responses into its transport presentation without leaking transport types into the core.
func levelActionResponses(actions []model.CaseTemplateLevelAction) []TemplateActionResponse {
	responses := make([]TemplateActionResponse, 0, len(actions))
	for _, action := range actions {
		responses = append(responses, TemplateActionResponse{
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
			Enabled:          action.Enabled,
		})
	}
	return responses
}

// parseJSON parses json and rejects malformed input before it reaches core logic.
func parseJSON(body string) any {
	if body == "" {
		return map[string]any{}
	}

	var value any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		return map[string]any{}
	}
	return value
}
