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

const (
	// MaxTemplateSafeRetries bounds the only execution control exposed to guild administrators.
	MaxTemplateSafeRetries = 10
	// MaxTimeoutDurationSeconds is Discord's maximum 28-day member timeout.
	MaxTimeoutDurationSeconds = 28 * 24 * 60 * 60
	// MaxBanDeleteMessageSeconds is Discord's maximum seven-day ban history deletion window.
	MaxBanDeleteMessageSeconds = 7 * 24 * 60 * 60
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
	Levels         []TemplateLevelInput `json:"levels"`
}

// TemplateLevelInput groups the validated inputs needed for template level input.
type TemplateLevelInput struct {
	Name             string                `json:"name"`
	Position         int                   `json:"position"`
	IsDefault        bool                  `json:"is_default"`
	TriggerCaseCount int                   `json:"trigger_case_count"`
	NotifyUser       bool                  `json:"notify_user"`
	Actions          []TemplateActionInput `json:"actions"`
}

// TemplateActionInput groups the validated inputs needed for template action input.
type TemplateActionInput struct {
	ActionType             model.ActionType `json:"action_type"`
	TimeoutDurationSeconds int              `json:"timeout_duration_seconds,omitempty"`
	DeleteMessageSeconds   int              `json:"delete_message_seconds,omitempty"`
	MaxRetries             int              `json:"max_retries"`
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
	NotifyUser       bool   `json:"notify_user"`
}

// TemplateLevelResponse is the transport-neutral representation returned for template level response.
type TemplateLevelResponse struct {
	TemplateLevelDetails
	Actions []TemplateActionResponse `json:"actions"`
}

// TemplateActionResponse is the transport-neutral representation returned for template action response.
type TemplateActionResponse struct {
	ID                     string           `json:"id"`
	ActionType             model.ActionType `json:"action_type"`
	TimeoutDurationSeconds int              `json:"timeout_duration_seconds,omitempty"`
	DeleteMessageSeconds   int              `json:"delete_message_seconds,omitempty"`
	MaxRetries             uint8            `json:"max_retries"`
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

	template := model.CaseTemplate{
		GuildID:                guildContext.Guild.ID,
		Slug:                   slug,
		Name:                   name,
		Description:            strings.TrimSpace(input.Description),
		ReasonTemplate:         reasonTemplate,
		Appealable:             input.Appealable,
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
	thresholds := make(map[int]struct{}, len(inputs))

	for i, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, validationError("level name is required")
		}
		if !input.IsDefault && input.TriggerCaseCount <= 0 {
			return nil, validationError("escalation level trigger_case_count must be positive")
		}
		if input.IsDefault {
			defaultCount++
			if input.TriggerCaseCount != 0 {
				return nil, validationError("default level cannot have escalation triggers")
			}
		} else {
			if _, duplicate := thresholds[input.TriggerCaseCount]; duplicate {
				return nil, validationError("escalation level trigger_case_count values must be distinct")
			}
			thresholds[input.TriggerCaseCount] = struct{}{}
		}
		actions, err := normalizeActions(input.Actions)
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
				NotifyUser:       input.NotifyUser,
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
func normalizeActions(inputs []TemplateActionInput) ([]model.CaseTemplateLevelAction, error) {
	if len(inputs) > 1 {
		return nil, validationError("a template level can contain at most one enforcement action")
	}
	actions := make([]model.CaseTemplateLevelAction, 0, len(inputs))
	for _, input := range inputs {
		if input.ActionType == "record_warning" {
			return nil, validationError("record_warning is not a template action; creating a case records the warning")
		}
		if input.ActionType == model.ActionSendDM {
			return nil, validationError("send_dm is not a template action; set notify_user on the level")
		}
		if !validActionType(input.ActionType) {
			return nil, validationError("action_type is invalid")
		}
		if input.MaxRetries < 0 || input.MaxRetries > MaxTemplateSafeRetries {
			return nil, validationError(fmt.Sprintf("max_retries must be between 0 and %d", MaxTemplateSafeRetries))
		}
		configJSON, err := normalizeTemplateActionConfig(input)
		if err != nil {
			return nil, err
		}

		actions = append(actions, model.CaseTemplateLevelAction{
			ActionType: input.ActionType,
			ConfigJSON: configJSON,
			MaxRetries: uint8(input.MaxRetries),
		})
	}

	return actions, nil
}

// templateActionConfig is the internal canonical snapshot for product-owned action settings.
type templateActionConfig struct {
	DurationSeconds      int `json:"duration_seconds,omitempty"`
	DeleteMessageSeconds int `json:"delete_message_seconds,omitempty"`
}

// normalizeTemplateActionConfig validates typed settings and stores only the setting owned by the selected action.
func normalizeTemplateActionConfig(input TemplateActionInput) (string, error) {
	config := templateActionConfig{}
	switch input.ActionType {
	case model.ActionTimeoutUser:
		if input.TimeoutDurationSeconds <= 0 || input.TimeoutDurationSeconds > MaxTimeoutDurationSeconds {
			return "", validationError(fmt.Sprintf("timeout_duration_seconds must be between 1 and %d", MaxTimeoutDurationSeconds))
		}
		if input.DeleteMessageSeconds != 0 {
			return "", validationError("delete_message_seconds is only valid for ban actions")
		}
		config.DurationSeconds = input.TimeoutDurationSeconds
	case model.ActionKickUser:
		if input.TimeoutDurationSeconds != 0 || input.DeleteMessageSeconds != 0 {
			return "", validationError("kick actions do not accept action settings")
		}
	case model.ActionBanUser:
		if input.TimeoutDurationSeconds != 0 {
			return "", validationError("timeout_duration_seconds is only valid for timeout actions")
		}
		if input.DeleteMessageSeconds < 0 || input.DeleteMessageSeconds > MaxBanDeleteMessageSeconds {
			return "", validationError(fmt.Sprintf("delete_message_seconds must be between 0 and %d", MaxBanDeleteMessageSeconds))
		}
		config.DeleteMessageSeconds = input.DeleteMessageSeconds
	}
	body, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshal template action config: %w", err)
	}
	return string(body), nil
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
		Version:                template.Version,
		CreatedByDiscordUserID: template.CreatedByDiscordUserID,
		UpdatedByDiscordUserID: template.UpdatedByDiscordUserID,
		ArchivedAt:             template.ArchivedAt,
		Levels:                 make([]TemplateLevelResponse, 0, len(expanded.Levels)),
	}

	for _, level := range expanded.Levels {
		levelResponse := TemplateLevelResponse{
			TemplateLevelDetails: templateLevelDetails(level.Level),
			Actions:              make([]TemplateActionResponse, 0, len(level.Actions)),
		}
		for _, action := range level.Actions {
			levelResponse.Actions = append(levelResponse.Actions, templateActionResponse(action))
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
		NotifyUser:       level.NotifyUser,
	}
}

// levelActionResponses converts level action responses into its transport presentation without leaking transport types into the core.
func levelActionResponses(actions []model.CaseTemplateLevelAction) []TemplateActionResponse {
	responses := make([]TemplateActionResponse, 0, len(actions))
	for _, action := range actions {
		responses = append(responses, templateActionResponse(action))
	}
	return responses
}

// templateActionResponse projects canonical or compatible stored settings into the typed product contract.
func templateActionResponse(action model.CaseTemplateLevelAction) TemplateActionResponse {
	config := decodeTemplateActionConfig(action.ConfigJSON)
	return TemplateActionResponse{
		ID:                     action.ID,
		ActionType:             action.ActionType,
		TimeoutDurationSeconds: config.DurationSeconds,
		DeleteMessageSeconds:   config.DeleteMessageSeconds,
		MaxRetries:             action.MaxRetries,
	}
}

// decodeTemplateActionConfig reads canonical settings and the previous duration-minutes representation without exposing it.
func decodeTemplateActionConfig(body string) templateActionConfig {
	var stored struct {
		DurationSeconds      int `json:"duration_seconds"`
		DurationMinutes      int `json:"duration_minutes"`
		DeleteMessageSeconds int `json:"delete_message_seconds"`
	}
	if err := json.Unmarshal([]byte(body), &stored); err != nil {
		return templateActionConfig{}
	}
	durationSeconds := stored.DurationSeconds
	if durationSeconds == 0 && stored.DurationMinutes > 0 {
		durationSeconds = stored.DurationMinutes * 60
	}
	return templateActionConfig{
		DurationSeconds:      durationSeconds,
		DeleteMessageSeconds: stored.DeleteMessageSeconds,
	}
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
