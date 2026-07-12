package quack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

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
	ErrTemplateValidation                  = errors.New("template validation failed")
	ErrTemplateNotFound                    = errors.New("case template not found")
	ErrTemplateCompatibilityReviewRequired = model.ErrTemplateCompatibilityReviewRequired
)

// TemplateCompatibilityReviewError aliases the domain error returned when preserved legacy policy cannot be projected as a valid live template.
type TemplateCompatibilityReviewError = model.TemplateCompatibilityReviewError

var templateSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,63}$`)

// TemplateService owns template validation, normalization, persistence, and audit creation.
type TemplateService struct {
	store Repository
}

// TemplateInput groups the validated inputs needed for template input.
type TemplateInput struct {
	Slug           string                      `json:"slug"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description"`
	ReasonTemplate string                      `json:"reason_template"`
	Appealable     bool                        `json:"appealable"`
	ContextFields  []TemplateContextFieldInput `json:"context_fields"`
	Levels         []TemplateLevelInput        `json:"levels"`
}

// TemplateContextFieldInput defines an ordered member-visible field collected during case creation.
type TemplateContextFieldInput struct {
	Key       string                 `json:"key"`
	Label     string                 `json:"label"`
	FieldType model.ContextFieldType `json:"type"`
	Position  int                    `json:"position"`
	Required  bool                   `json:"required"`
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
	ID                     string                         `json:"id"`
	GuildID                string                         `json:"guild_id"`
	Slug                   string                         `json:"slug"`
	Name                   string                         `json:"name"`
	Description            string                         `json:"description"`
	ReasonTemplate         string                         `json:"reason_template"`
	Appealable             bool                           `json:"appealable"`
	Version                uint                           `json:"version"`
	CreatedByDiscordUserID string                         `json:"created_by_discord_user_id"`
	UpdatedByDiscordUserID string                         `json:"updated_by_discord_user_id"`
	ArchivedAt             *time.Time                     `json:"archived_at"`
	ContextFields          []TemplateContextFieldResponse `json:"context_fields"`
	Levels                 []TemplateLevelResponse        `json:"levels"`
}

// TemplateContextFieldResponse is the stable transport representation of a template context definition.
type TemplateContextFieldResponse struct {
	ID        string                 `json:"id"`
	Key       string                 `json:"key"`
	Label     string                 `json:"label"`
	FieldType model.ContextFieldType `json:"type"`
	Position  int                    `json:"position"`
	Required  bool                   `json:"required"`
}

// TemplatePolicy is the guild-neutral policy-only import and export shape.
type TemplatePolicy struct {
	SchemaVersion  int                         `json:"schema_version"`
	Slug           string                      `json:"slug"`
	Name           string                      `json:"name"`
	Description    string                      `json:"description"`
	OfficialReason string                      `json:"official_reason"`
	Appealable     bool                        `json:"appealable"`
	ContextFields  []TemplateContextFieldInput `json:"context_fields"`
	Levels         []TemplateLevelInput        `json:"levels"`
}

// TemplateImportInput requires explicit confirmation before imported policy becomes active.
type TemplateImportInput struct {
	Confirm bool           `json:"confirm"`
	Policy  TemplatePolicy `json:"policy"`
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

// ListActive returns only templates currently available for new cases and Discord autocomplete.
func (s *TemplateService) ListActive(ctx context.Context, guildContext *GuildStaffContext) ([]TemplateResponse, error) {
	all, err := s.List(ctx, guildContext)
	if err != nil {
		return nil, err
	}
	active := make([]TemplateResponse, 0, len(all))
	for _, item := range all {
		if item.ArchivedAt == nil {
			active = append(active, item)
		}
	}
	return active, nil
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
		Template:      normalized.template,
		ContextFields: normalized.contextFields,
		Levels:        normalized.levels,
		Audit:         s.auditEntry(ctx, guildContext, "case_template.create", "case_template", "", model.AuditResultSuccess, ""),
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
		GuildID:       guildContext.Guild.ID,
		TemplateID:    templateID,
		Template:      normalized.template,
		ContextFields: normalized.contextFields,
		Levels:        normalized.levels,
		Audit:         s.auditEntry(ctx, guildContext, "case_template.update", "case_template", templateID, model.AuditResultSuccess, ""),
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

// Restore reverses archive without changing the template identity or version.
func (s *TemplateService) Restore(ctx context.Context, guildContext *GuildStaffContext, templateID string) (*TemplateResponse, error) {
	ctx = ensureTraceContext(ctx)
	expanded, err := s.store.RestoreCaseTemplate(ctx, guildContext.Guild.ID, strings.TrimSpace(templateID), s.auditEntry(ctx, guildContext, "case_template.restore", "case_template", templateID, model.AuditResultSuccess, ""))
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.restore", "case_template", templateID, model.AuditResultFailure, err.Error())
		return nil, err
	}
	if expanded == nil {
		_ = s.audit(ctx, guildContext, "case_template.restore", "case_template", templateID, model.AuditResultFailure, ErrTemplateNotFound.Error())
		return nil, ErrTemplateNotFound
	}
	response := templateResponse(*expanded)
	return &response, nil
}

// Export returns policy fields only, deliberately excluding guild identity, history, channels, audit data, and secrets.
func (s *TemplateService) Export(ctx context.Context, guildContext *GuildStaffContext, templateID string) (*TemplatePolicy, error) {
	template, err := s.Get(ctx, guildContext, templateID)
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.export", "case_template", templateID, model.AuditResultFailure, err.Error())
		return nil, err
	}
	policy := &TemplatePolicy{SchemaVersion: 1, Slug: template.Slug, Name: template.Name, Description: template.Description, OfficialReason: template.ReasonTemplate, Appealable: template.Appealable}
	for _, f := range template.ContextFields {
		policy.ContextFields = append(policy.ContextFields, TemplateContextFieldInput{Key: f.Key, Label: f.Label, FieldType: f.FieldType, Position: f.Position, Required: f.Required})
	}
	for _, level := range template.Levels {
		in := TemplateLevelInput{Name: level.Name, Position: level.Position, IsDefault: level.IsDefault, TriggerCaseCount: level.TriggerCaseCount, NotifyUser: level.NotifyUser}
		for _, action := range level.Actions {
			in.Actions = append(in.Actions, TemplateActionInput{ActionType: action.ActionType, TimeoutDurationSeconds: action.TimeoutDurationSeconds, DeleteMessageSeconds: action.DeleteMessageSeconds, MaxRetries: int(action.MaxRetries)})
		}
		policy.Levels = append(policy.Levels, in)
	}
	if err := s.audit(ctx, guildContext, "case_template.export", "case_template", templateID, model.AuditResultSuccess, ""); err != nil {
		return nil, err
	}
	return policy, nil
}

// Import validates confirmed guild-neutral policy and creates a new active guild-owned template identity.
func (s *TemplateService) Import(ctx context.Context, guildContext *GuildStaffContext, input TemplateImportInput) (*TemplateResponse, error) {
	if !input.Confirm {
		err := validationError("template import must be explicitly confirmed")
		_ = s.audit(ctx, guildContext, "case_template.import", "case_template", "unknown", model.AuditResultFailure, err.Error())
		return nil, err
	}
	if input.Policy.SchemaVersion != 1 {
		err := validationError("unsupported template policy schema_version")
		_ = s.audit(ctx, guildContext, "case_template.import", "case_template", "unknown", model.AuditResultFailure, err.Error())
		return nil, err
	}
	normalized, err := s.validate(ctx, guildContext, "", TemplateInput{Slug: input.Policy.Slug, Name: input.Policy.Name, Description: input.Policy.Description, ReasonTemplate: input.Policy.OfficialReason, Appealable: input.Policy.Appealable, ContextFields: input.Policy.ContextFields, Levels: input.Policy.Levels})
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.import", "case_template", "unknown", model.AuditResultFailure, err.Error())
		return nil, err
	}
	expanded, err := s.store.CreateCaseTemplate(ctx, model.CreateCaseTemplateParams{Template: normalized.template, ContextFields: normalized.contextFields, Levels: normalized.levels, Audit: s.auditEntry(ctx, guildContext, "case_template.import", "case_template", "", model.AuditResultSuccess, "")})
	if err != nil {
		_ = s.audit(ctx, guildContext, "case_template.import", "case_template", "unknown", model.AuditResultFailure, err.Error())
		return nil, err
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
		_ = s.audit(ctx, guildContext, "case_template.archive", "case_template", templateID, model.AuditResultFailure, err.Error())
		return nil, err
	}
	if expanded == nil {
		_ = s.audit(ctx, guildContext, "case_template.archive", "case_template", templateID, model.AuditResultFailure, ErrTemplateNotFound.Error())
		return nil, ErrTemplateNotFound
	}

	response := templateResponse(*expanded)
	return &response, nil
}

// normalizedTemplate groups the normalized template state used to keep this package's responsibilities explicit.
type normalizedTemplate struct {
	template      model.CaseTemplate
	contextFields []model.CaseTemplateContextField
	levels        []model.ExpandedCaseTemplateLevel
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

	contextFields, err := normalizeContextFields(input.ContextFields)
	if err != nil {
		return nil, err
	}
	return &normalizedTemplate{template: template, contextFields: contextFields, levels: levels}, nil
}

// normalizeContextFields validates identifiers, types, unique ordering, and member-visible field bounds.
func normalizeContextFields(inputs []TemplateContextFieldInput) ([]model.CaseTemplateContextField, error) {
	if len(inputs) > 10 {
		return nil, validationError("at most 10 context fields are allowed")
	}
	keys := map[string]struct{}{}
	positions := map[int]struct{}{}
	fields := make([]model.CaseTemplateContextField, 0, len(inputs))
	for i, input := range inputs {
		key := strings.ToLower(strings.TrimSpace(input.Key))
		if !templateSlugPattern.MatchString(key) {
			return nil, validationError("context field key must be 2-64 lowercase letters, numbers, underscores, or hyphens")
		}
		if _, ok := keys[key]; ok {
			return nil, validationError("context field keys must be unique")
		}
		keys[key] = struct{}{}
		label := strings.TrimSpace(input.Label)
		if label == "" || len([]rune(label)) > 100 {
			return nil, validationError("context field label must be 1-100 characters")
		}
		if !validContextFieldType(input.FieldType) {
			return nil, validationError("context field type is invalid")
		}
		position := input.Position
		if position == 0 {
			position = i + 1
		}
		if position < 1 {
			return nil, validationError("context field position must be positive")
		}
		if _, ok := positions[position]; ok {
			return nil, validationError("context field positions must be unique")
		}
		positions[position] = struct{}{}
		fields = append(fields, model.CaseTemplateContextField{Key: key, Label: label, FieldType: input.FieldType, Position: position, Required: input.Required})
	}
	return fields, nil
}

// validContextFieldType reports whether a field uses one of the five v5 value shapes.
func validContextFieldType(value model.ContextFieldType) bool {
	switch value {
	case model.ContextFieldShortText, model.ContextFieldLongText, model.ContextFieldBoolean, model.ContextFieldNumber, model.ContextFieldMessageLink:
		return true
	default:
		return false
	}
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
		ContextFields:          make([]TemplateContextFieldResponse, 0, len(expanded.ContextFields)),
		Levels:                 make([]TemplateLevelResponse, 0, len(expanded.Levels)),
	}
	for _, field := range expanded.ContextFields {
		response.ContextFields = append(response.ContextFields, TemplateContextFieldResponse{ID: field.ID, Key: field.Key, Label: field.Label, FieldType: field.FieldType, Position: field.Position, Required: field.Required})
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
