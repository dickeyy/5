package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

var (
	ErrTemplateValidation = errors.New("template validation failed")
	ErrTemplateNotFound   = errors.New("case template not found")
)

var templateSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

type TemplateService struct {
	store *storage.Store
}

type TemplateInput struct {
	Slug                   string                `json:"slug"`
	Name                   string                `json:"name"`
	Description            string                `json:"description"`
	ReasonTemplate         string                `json:"reason_template"`
	Appealable             bool                  `json:"appealable"`
	Enabled                *bool                 `json:"enabled"`
	Levels                 []TemplateLevelInput  `json:"levels"`
	RequiredPermissionBits uint64                `json:"required_permission_bits"`
	DefaultWeight          int                   `json:"default_weight"`
	Actions                []TemplateActionInput `json:"actions"`
	EscalationRules        []EscalationRuleInput `json:"escalation_rules"`
}

type TemplateLevelInput struct {
	Name                 string                `json:"name"`
	Position             int                   `json:"position"`
	IsDefault            bool                  `json:"is_default"`
	TriggerCaseCount     int                   `json:"trigger_case_count"`
	WindowMinutes        int                   `json:"window_minutes"`
	Enabled              *bool                 `json:"enabled"`
	Actions              []TemplateActionInput `json:"actions"`
	TriggerWeightTotal   int                   `json:"trigger_weight_total"`
	EscalateToTemplateID *string               `json:"escalate_to_template_id"`
}

type TemplateActionInput struct {
	ActionType             structs.ActionType `json:"action_type"`
	Config                 json.RawMessage    `json:"config"`
	ContinueOnError        bool               `json:"continue_on_error"`
	MaxRetries             int                `json:"max_retries"`
	RetryBackoffMS         int                `json:"retry_backoff_ms"`
	TimeoutMS              int                `json:"timeout_ms"`
	IdempotencyScope       string             `json:"idempotency_scope"`
	Enabled                *bool              `json:"enabled"`
	RequiredPermissionBits uint64             `json:"required_permission_bits"`
}

type EscalationRuleInput struct {
	Name                 string                  `json:"name"`
	Scope                structs.EscalationScope `json:"scope"`
	Priority             int                     `json:"priority"`
	TriggerCaseCount     int                     `json:"trigger_case_count"`
	TriggerWeightTotal   int                     `json:"trigger_weight_total"`
	WindowMinutes        int                     `json:"window_minutes"`
	EscalateToTemplateID *string                 `json:"escalate_to_template_id"`
	RuleConfig           json.RawMessage         `json:"rule_config"`
	Enabled              *bool                   `json:"enabled"`
	StopAfterMatch       *bool                   `json:"stop_after_match"`
}

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

type TemplateLevelDetails struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Position         int    `json:"position"`
	IsDefault        bool   `json:"is_default"`
	TriggerCaseCount int    `json:"trigger_case_count"`
	WindowMinutes    int    `json:"window_minutes"`
}

type TemplateLevelResponse struct {
	TemplateLevelDetails
	Enabled bool                     `json:"enabled"`
	Actions []TemplateActionResponse `json:"actions"`
}

type TemplateActionResponse struct {
	ID               string             `json:"id"`
	Position         int                `json:"position"`
	ActionType       structs.ActionType `json:"action_type"`
	Config           any                `json:"config"`
	ContinueOnError  bool               `json:"continue_on_error"`
	MaxRetries       uint8              `json:"max_retries"`
	RetryBackoffMS   int                `json:"retry_backoff_ms"`
	TimeoutMS        int                `json:"timeout_ms"`
	IdempotencyScope string             `json:"idempotency_scope"`
	Enabled          bool               `json:"enabled"`
}

func NewTemplateService(store *storage.Store) *TemplateService {
	return &TemplateService{store: store}
}

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

func (s *TemplateService) Create(ctx context.Context, guildContext *GuildStaffContext, input TemplateInput) (*TemplateResponse, error) {
	normalized, err := s.validate(ctx, guildContext, "", input)
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.create", "case_template", "unknown", structs.AuditResultFailure, err.Error())
		return nil, err
	}

	expanded, err := s.store.CreateCaseTemplate(ctx, storage.CreateCaseTemplateParams{
		Template: normalized.template,
		Levels:   normalized.levels,
		Audit:    s.auditEntry(guildContext, "case_template.create", "case_template", "", structs.AuditResultSuccess, ""),
	})
	if err != nil {
		return nil, err
	}

	response := templateResponse(*expanded)
	return &response, nil
}

func (s *TemplateService) Update(ctx context.Context, guildContext *GuildStaffContext, templateID string, input TemplateInput) (*TemplateResponse, error) {
	existing, err := s.store.GetCaseTemplateExpanded(ctx, guildContext.Guild.ID, templateID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrTemplateNotFound
	}

	normalized, err := s.validate(ctx, guildContext, templateID, input)
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.update", "case_template", templateID, structs.AuditResultFailure, err.Error())
		return nil, err
	}

	expanded, err := s.store.UpdateCaseTemplate(ctx, storage.UpdateCaseTemplateParams{
		GuildID:    guildContext.Guild.ID,
		TemplateID: templateID,
		Template:   normalized.template,
		Levels:     normalized.levels,
		Audit:      s.auditEntry(guildContext, "case_template.update", "case_template", templateID, structs.AuditResultSuccess, ""),
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

func (s *TemplateService) Archive(ctx context.Context, guildContext *GuildStaffContext, templateID string) (*TemplateResponse, error) {
	expanded, err := s.store.ArchiveCaseTemplate(
		ctx,
		guildContext.Guild.ID,
		templateID,
		s.auditEntry(guildContext, "case_template.archive", "case_template", templateID, structs.AuditResultSuccess, ""),
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

type normalizedTemplate struct {
	template structs.CaseTemplate
	levels   []storage.ExpandedCaseTemplateLevel
}

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

	if input.RequiredPermissionBits != 0 {
		return nil, validationError("required_permission_bits is not part of foundation templates")
	}
	if input.DefaultWeight != 0 {
		return nil, validationError("default_weight is not part of foundation templates")
	}
	if len(input.Actions) > 0 {
		return nil, validationError("flat template actions are not supported; use levels[].actions")
	}
	if len(input.EscalationRules) > 0 {
		return nil, validationError("escalation_rules are not supported; use levels")
	}

	levels, err := normalizeLevels(input.Levels)
	if err != nil {
		return nil, err
	}

	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	template := structs.CaseTemplate{
		GuildID:                guildContext.Guild.ID,
		Slug:                   slug,
		Name:                   name,
		Description:            strings.TrimSpace(input.Description),
		ReasonTemplate:         reasonTemplate,
		DefaultSeverity:        structs.CaseSeverityMedium,
		DefaultWeight:          1,
		Appealable:             input.Appealable,
		Enabled:                enabled,
		CreatedByDiscordUserID: guildContext.Staff.DiscordUserID,
		UpdatedByDiscordUserID: guildContext.Staff.DiscordUserID,
	}

	return &normalizedTemplate{template: template, levels: levels}, nil
}

func normalizeLevels(inputs []TemplateLevelInput) ([]storage.ExpandedCaseTemplateLevel, error) {
	if len(inputs) == 0 {
		return nil, validationError("at least one level is required")
	}

	levels := make([]storage.ExpandedCaseTemplateLevel, 0, len(inputs))
	defaultCount := 0
	defaultEnabledActionCount := 0

	for i, input := range inputs {
		name := strings.TrimSpace(input.Name)
		if name == "" {
			return nil, validationError("level name is required")
		}
		if input.TriggerWeightTotal != 0 {
			return nil, validationError("trigger_weight_total is not part of foundation escalation")
		}
		if input.EscalateToTemplateID != nil && strings.TrimSpace(*input.EscalateToTemplateID) != "" {
			return nil, validationError("escalate_to_template_id is not part of foundation escalation")
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

		enabled := true
		if input.Enabled != nil {
			enabled = *input.Enabled
		}
		actions, enabledActionCount, err := normalizeActions(input.Actions)
		if err != nil {
			return nil, err
		}
		if input.IsDefault {
			defaultEnabledActionCount = enabledActionCount
		}

		position := input.Position
		if position == 0 {
			position = i + 1
		}
		if position < 0 {
			return nil, validationError("level position must be non-negative")
		}

		levels = append(levels, storage.ExpandedCaseTemplateLevel{
			Level: structs.CaseTemplateLevel{
				Position:         position,
				Name:             name,
				IsDefault:        input.IsDefault,
				TriggerCaseCount: input.TriggerCaseCount,
				WindowMinutes:    input.WindowMinutes,
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
	if defaultEnabledActionCount == 0 {
		return nil, validationError("default level requires at least one enabled action")
	}

	return levels, nil
}

func normalizeActions(inputs []TemplateActionInput) ([]structs.CaseTemplateLevelAction, int, error) {
	actions := make([]structs.CaseTemplateLevelAction, 0, len(inputs))
	enabledCount := 0
	for i, input := range inputs {
		if input.RequiredPermissionBits != 0 {
			return nil, 0, validationError("action required_permission_bits is not part of foundation templates")
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

		actions = append(actions, structs.CaseTemplateLevelAction{
			Position:         i + 1,
			ActionType:       input.ActionType,
			ConfigJSON:       configJSON,
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

func validActionType(actionType structs.ActionType) bool {
	switch actionType {
	case structs.ActionRecordWarning, structs.ActionSendDM, structs.ActionTimeoutUser, structs.ActionKickUser, structs.ActionBanUser:
		return true
	default:
		return false
	}
}

func validationError(message string) error {
	return fmt.Errorf("%w: %s", ErrTemplateValidation, message)
}

func (s *TemplateService) audit(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result structs.AuditResult, failureReason string) error {
	entry := s.auditEntry(guildContext, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return s.store.CreateAuditLogEntry(ctx, entry)
}

func (s *TemplateService) auditEntry(guildContext *GuildStaffContext, action, resourceType, resourceID string, result structs.AuditResult, failureReason string) *structs.AuditLogEntry {
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

func templateResponse(expanded storage.ExpandedCaseTemplate) TemplateResponse {
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

func templateLevelDetails(level structs.CaseTemplateLevel) TemplateLevelDetails {
	return TemplateLevelDetails{
		ID:               level.ID,
		Name:             level.Name,
		Position:         level.Position,
		IsDefault:        level.IsDefault,
		TriggerCaseCount: level.TriggerCaseCount,
		WindowMinutes:    level.WindowMinutes,
	}
}

func enabledLevelActions(actions []structs.CaseTemplateLevelAction) []structs.CaseTemplateLevelAction {
	enabled := make([]structs.CaseTemplateLevelAction, 0, len(actions))
	for _, action := range actions {
		if action.Enabled {
			enabled = append(enabled, action)
		}
	}
	return enabled
}

func levelActionResponses(actions []structs.CaseTemplateLevelAction) []TemplateActionResponse {
	responses := make([]TemplateActionResponse, 0, len(actions))
	for _, action := range actions {
		responses = append(responses, TemplateActionResponse{
			ID:               action.ID,
			Position:         action.Position,
			ActionType:       action.ActionType,
			Config:           parseJSON(action.ConfigJSON),
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
