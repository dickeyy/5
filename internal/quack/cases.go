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
	errCasePreflightStale       = errors.New("case preflight became stale")
)

// CaseService owns case authorization, escalation selection, snapshots, auditing, and action scheduling.
type CaseService struct {
	store      Repository
	scheduler  CaseWorkScheduler
	authorizer *GuildService
	evidence   *EvidenceService
}

// CaseInput groups the validated inputs needed for case input.
type CaseInput struct {
	TemplateID              string                  `json:"template_id"`
	TargetDiscordUserID     string                  `json:"target_discord_user_id"`
	Source                  model.CaseSource        `json:"source"`
	ContextChannelDiscordID string                  `json:"context_channel_discord_id"`
	ContextMessageDiscordID string                  `json:"context_message_discord_id"`
	ContextURL              string                  `json:"context_url"`
	Metadata                json.RawMessage         `json:"metadata"`
	ContextValues           []CaseContextValueInput `json:"context_values"`
	EvidenceLinks           []string                `json:"evidence_links"`
	ReplacesCaseID          string                  `json:"replaces_case_id,omitempty"`
	IdempotencyKey          string                  `json:"-"`
}

// CaseContextValueInput carries one typed value keyed by its template definition.
type CaseContextValueInput struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// CaseContextValueResponse is the immutable member-visible definition/value pair stored with the case.
type CaseContextValueResponse struct {
	Key       string                 `json:"key"`
	Label     string                 `json:"label"`
	FieldType model.ContextFieldType `json:"type"`
	Required  bool                   `json:"required"`
	Value     any                    `json:"value"`
}

// CaseListInput groups the validated inputs needed for case list input.
type CaseListInput struct {
	Limit                  string
	Offset                 string
	TargetDiscordUserID    string
	ModeratorDiscordUserID string
	TemplateID             string
	Validity               string
	CaseNumber             string
	ActionResult           string
	AppealStatus           string
	CreatedAfter           string
	CreatedBefore          string
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
	Evidence         []CaseEvidenceResponse        `json:"evidence"`
	Notification     *CaseNotificationResponse     `json:"notification,omitempty"`
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
	CreatedAt               time.Time                  `json:"created_at"`
	UpdatedAt               time.Time                  `json:"updated_at"`
	ID                      string                     `json:"id"`
	GuildID                 string                     `json:"guild_id"`
	CaseNumber              uint64                     `json:"case_number"`
	TemplateID              *string                    `json:"template_id"`
	TemplateVersion         uint                       `json:"template_version"`
	TargetDiscordUserID     string                     `json:"target_discord_user_id"`
	ModeratorDiscordUserID  string                     `json:"moderator_discord_user_id"`
	Reason                  string                     `json:"reason"`
	Validity                model.CaseValidity         `json:"validity"`
	Source                  model.CaseSource           `json:"source"`
	ContextChannelDiscordID string                     `json:"context_channel_discord_id,omitempty"`
	ContextMessageDiscordID string                     `json:"context_message_discord_id,omitempty"`
	ContextURL              string                     `json:"context_url,omitempty"`
	Metadata                any                        `json:"metadata"`
	ContextValues           []CaseContextValueResponse `json:"context_values"`
	VoidedReason            string                     `json:"voided_reason,omitempty"`
	VoidedAt                *time.Time                 `json:"voided_at,omitempty"`
	ReplacementCaseID       *string                    `json:"replacement_case_id,omitempty"`
	ReplacesCaseID          *string                    `json:"replaces_case_id,omitempty"`
	SelectedLevel           *CaseSelectedLevel         `json:"selected_level,omitempty"`
	Actions                 []CaseActionResponse       `json:"actions"`
}

// CaseEvidenceAttachmentResponse exposes stable evidence references without leaking the staff-only channel identity.
type CaseEvidenceAttachmentResponse struct {
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	OriginalURL  string `json:"original_url"`
	PreservedURL string `json:"preserved_url,omitempty"`
	CopyOutcome  string `json:"copy_outcome"`
	Warning      string `json:"warning,omitempty"`
	SizeBytes    int64  `json:"size_bytes"`
}

// CaseEvidenceResponse exposes an immutable message snapshot and bounded attachment metadata.
type CaseEvidenceResponse struct {
	ID                  string                           `json:"id"`
	AuthorDiscordUserID string                           `json:"author_discord_user_id"`
	MessageURL          string                           `json:"message_url"`
	Content             string                           `json:"content"`
	CaptureOutcome      string                           `json:"capture_outcome"`
	CaptureWarning      string                           `json:"capture_warning,omitempty"`
	MessageCreatedAt    time.Time                        `json:"message_created_at"`
	MessageEditedAt     *time.Time                       `json:"message_edited_at,omitempty"`
	Embeds              any                              `json:"embeds"`
	Attachments         []CaseEvidenceAttachmentResponse `json:"attachments"`
}

// CaseNotificationResponse exposes delivery state separately from enforcement.
type CaseNotificationResponse struct {
	Status        model.NotificationStatus `json:"status"`
	AttemptCount  uint8                    `json:"attempt_count"`
	LastErrorCode string                   `json:"last_error_code,omitempty"`
	LastError     string                   `json:"last_error,omitempty"`
	SentAt        *time.Time               `json:"sent_at,omitempty"`
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
	Template      templateSnapshotTemplate       `json:"template"`
	SelectedLevel CaseSelectedLevel              `json:"selected_level"`
	Actions       []templateSnapshotAction       `json:"actions"`
	ContextFields []TemplateContextFieldResponse `json:"context_fields"`
	ContextValues []CaseContextValueResponse     `json:"context_values"`
}

// templateSnapshot groups the template snapshot state used to keep this package's responsibilities explicit.
type templateSnapshot struct {
	Template      templateSnapshotTemplate       `json:"template"`
	SelectedLevel CaseSelectedLevel              `json:"selected_level"`
	Actions       []templateSnapshotAction       `json:"actions"`
	ContextFields []TemplateContextFieldResponse `json:"context_fields"`
	ContextValues []CaseContextValueResponse     `json:"context_values"`
}

// templateSnapshotTemplate groups the template snapshot template state used to keep this package's responsibilities explicit.
type templateSnapshotTemplate struct {
	ID             string `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Version        uint   `json:"version"`
	ReasonTemplate string `json:"reason_template"`
	Appealable     bool   `json:"appealable"`
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

type caseCreatePreflight struct {
	TemplateID, SelectedLevelID, ContextValuesJSON string
	TemplateVersion                                uint
	ActionType                                     model.ActionType
	Captured                                       CapturedEvidence
}

// caseCreateAttribution distinguishes a live staff request from the one
// product-approved system automation path without widening generic case APIs.
type caseCreateAttribution struct {
	actorType   string
	auditSource model.AuditSource
	system      bool
}

// NewCaseService constructs case service with required dependencies explicit so callers control lifecycle and substitution.
func NewCaseService(store Repository, scheduler ...CaseWorkScheduler) *CaseService {
	service := &CaseService{store: store}
	if len(scheduler) > 0 {
		service.scheduler = scheduler[0]
	}
	return service
}

// WithEvidenceCapture configures the shared pre-commit evidence service.
func (s *CaseService) WithEvidenceCapture(evidence *EvidenceService) *CaseService {
	if s != nil {
		s.evidence = evidence
	}
	return s
}

// Create applies a staff-attributed template to a user inside the guild-scoped
// transaction boundary. The lock keeps escalation history and case numbering
// consistent, while scheduling occurs only after the transaction commits.
func (s *CaseService) Create(ctx context.Context, guildContext *GuildStaffContext, input CaseInput) (*CaseResponse, error) {
	if input.Source == model.CaseSourceHoneypot {
		return nil, validationCaseError("honeypot cases require the system application boundary")
	}
	return s.createWithAttribution(ctx, guildContext, input, caseCreateAttribution{actorType: "staff", auditSource: model.AuditSourceAPI})
}

// CreateSystemHoneypot applies one honeypot template through the ordinary case
// transaction while attributing the operation to Quack itself. It is intended
// only for the injected optional-module adapter and rejects every other source.
func (s *CaseService) CreateSystemHoneypot(ctx context.Context, guildID string, input CaseInput) (*CaseResponse, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if s.authorizer == nil {
		return nil, ErrAuthorizationUnavailable
	}
	if input.Source != model.CaseSourceHoneypot {
		return nil, validationCaseError("system case creation is restricted to the honeypot source")
	}
	guild, err := s.store.GetGuildByID(ctx, strings.TrimSpace(guildID))
	if err != nil {
		return nil, err
	}
	if guild == nil || !guild.IsActive {
		return nil, validationCaseError("active guild is required")
	}
	systemContext := &GuildStaffContext{
		Guild: guild,
		Staff: &model.StaffMember{},
		Permissions: map[model.PermissionAction]bool{
			model.PermissionActionCaseCreate: true,
		},
	}
	return s.createWithAttribution(ctx, systemContext, input, caseCreateAttribution{actorType: "system", auditSource: model.AuditSourceSystem, system: true})
}

// createWithAttribution owns the shared moderation path for staff and the
// narrowly scoped honeypot system boundary.
func (s *CaseService) createWithAttribution(ctx context.Context, guildContext *GuildStaffContext, input CaseInput, attribution caseCreateAttribution) (*CaseResponse, error) {
	ctx = ensureTraceContext(ctx)
	ctx = ContextWithAuditSource(ctx, AuditSourceForCaseSource(input.Source))
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil {
		return nil, validationCaseError("missing guild context")
	}
	key := strings.TrimSpace(input.IdempotencyKey)
	if len(key) > 191 {
		return nil, validationCaseError("idempotency key is too long")
	}
	input.IdempotencyKey = key
	if key != "" {
		existing, getErr := s.store.GetCaseByIdempotencyKey(ctx, guildContext.Guild.ID, key)
		if getErr != nil {
			return nil, getErr
		}
		if existing != nil {
			if existing.TargetDiscordUserID != strings.TrimSpace(input.TargetDiscordUserID) || (existing.TemplateID != nil && *existing.TemplateID != strings.TrimSpace(input.TemplateID)) {
				return nil, validationCaseError("idempotency key was already used for another case request")
			}
			actions, listErr := s.store.ListCaseActionExecutions(ctx, existing.ID)
			if listErr != nil {
				return nil, listErr
			}
			response := caseResponseFromModel(*existing, actions)
			return &response, nil
		}
	}
	var created *model.CreatedCase
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		var preflight *caseCreatePreflight
		preflight, err = s.preflightCreate(ctx, guildContext, input, attribution)
		if err != nil {
			break
		}
		err = s.store.WithGuildCaseLock(ctx, guildContext.Guild.ID, func(transactionalStore Repository) error {
			transactionalService := *s
			transactionalService.store = transactionalStore
			var createErr error
			created, createErr = transactionalService.create(ctx, guildContext, input, preflight, attribution)
			return createErr
		})
		if errors.Is(err, errCasePreflightStale) {
			continue
		}
		break
	}
	if err != nil {
		var authorizationErr *AuthorizationError
		if errors.As(err, &authorizationErr) && s.authorizer != nil {
			_ = s.authorizer.auditAuthorizationDenialWithMetadata(ctx, guildContext, authorizationErr.Capability, AuditSourceFromContext(ctx), authorizationErr.Reason, authorizationErr.MetadataJSON)
		}
		if errors.Is(err, ErrCaseValidation) || errors.Is(err, ErrCasePermissionDenied) || errors.Is(err, ErrCaseTemplateNotAvailable) || errors.Is(err, errCasePreflightStale) {
			_ = s.auditWithAttribution(ctx, guildContext, attribution, "case.create", "case", "unknown", model.AuditResultFailure, err.Error())
		}
		if errors.Is(err, errCasePreflightStale) {
			err = validationCaseError("case state changed repeatedly; retry the request")
		}
		return nil, err
	}

	if s.scheduler != nil {
		s.scheduler.Submit(ctx, created.Case.ID)
	}

	response := caseResponse(*created)
	return &response, nil
}

// preflightCreate performs live authorization and evidence capture before the atomic case transaction.
func (s *CaseService) preflightCreate(ctx context.Context, guildContext *GuildStaffContext, input CaseInput, attribution caseCreateAttribution) (*caseCreatePreflight, error) {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil || !guildContext.Can(model.PermissionActionCaseCreate) {
		return nil, ErrCasePermissionDenied
	}
	templateID, targetID := strings.TrimSpace(input.TemplateID), strings.TrimSpace(input.TargetDiscordUserID)
	if templateID == "" || targetID == "" {
		return nil, validationCaseError("template_id and target_discord_user_id are required")
	}
	template, err := s.store.GetCaseTemplateExpanded(ctx, guildContext.Guild.ID, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil || template.Template.ArchivedAt != nil {
		return nil, ErrCaseTemplateNotAvailable
	}
	selected, err := s.selectTemplateLevel(ctx, guildContext.Guild.ID, targetID, template)
	if err != nil {
		return nil, err
	}
	actionType := model.ActionType("")
	if len(selected.Actions) == 1 {
		actionType = selected.Actions[0].ActionType
	}
	if s.authorizer != nil {
		var err error
		if attribution.system {
			err = s.authorizer.PreflightSystemCase(ctx, guildContext, targetID, actionType)
		} else {
			err = s.authorizer.PreflightCase(ctx, guildContext, targetID, actionType)
		}
		if err != nil {
			return nil, err
		}
	}
	valuesJSON, links, hasFallback, err := validateCaseContextValues(template.ContextFields, input.ContextValues)
	if err != nil {
		return nil, err
	}
	links = append(links, input.EvidenceLinks...)
	if strings.TrimSpace(input.ContextURL) != "" {
		links = append(links, input.ContextURL)
	}
	result := &caseCreatePreflight{TemplateID: template.Template.ID, TemplateVersion: template.Template.Version, SelectedLevelID: selected.Level.ID, ActionType: actionType, ContextValuesJSON: valuesJSON}
	if len(links) > 0 {
		if s.evidence == nil {
			return nil, validationCaseError("evidence capture is not configured")
		}
		settings, settingsErr := s.store.GetGuildSettings(ctx, guildContext.Guild.ID)
		if settingsErr != nil {
			return nil, settingsErr
		}
		channelID := ""
		if settings != nil {
			channelID = settings.ManagedEvidenceChannelDiscordID
		}
		captured, captureErr := s.evidence.Capture(ctx, guildContext.Guild.DiscordGuildID, targetID, channelID, links, hasFallback)
		if captureErr != nil {
			_ = s.auditWithAttribution(ctx, guildContext, attribution, "evidence.capture", "case_evidence", "unknown", model.AuditResultFailure, captureErr.Error())
			return nil, validationCaseError(captureErr.Error())
		}
		if captured != nil {
			result.Captured = *captured
		}
	}
	return result, nil
}

// List returns list subject to authorization, ordering, and filtering constraints.
func (s *CaseService) List(ctx context.Context, guildContext *GuildStaffContext, input CaseListInput) (*CaseListResponse, error) {
	params, limit, offset, err := s.caseListParams(guildContext, input)
	if err != nil {
		if errors.Is(err, ErrCasePermissionDenied) {
			_ = s.audit(ctx, guildContext, string(model.AuditActionCaseSearch), "case", "list", model.AuditResultDenied, "permission_denied")
		}
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
	if err := s.audit(ctx, guildContext, "case.search", "case", "list", model.AuditResultSuccess, ""); err != nil {
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
		_ = s.audit(ctx, guildContext, string(model.AuditActionCaseRead), "case", strings.TrimSpace(caseRef), model.AuditResultDenied, "permission_denied")
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

	evidence, attachments, err := s.store.ListCaseEvidence(ctx, caseModel.ID)
	if err != nil {
		return nil, err
	}
	notification, err := s.store.GetCaseNotification(ctx, caseModel.ID)
	if err != nil {
		return nil, err
	}
	base := caseResponseFromModel(*caseModel, actions)
	if err := s.audit(ctx, guildContext, "case.read", "case", caseModel.ID, model.AuditResultSuccess, ""); err != nil {
		return nil, err
	}
	return &CaseDetailResponse{
		CaseResponse:     base,
		TemplateSnapshot: templateSnapshotResponse(caseModel.TemplateSnapshotJSON),
		Actions:          caseActionDetailResponses(actions, attempts),
		Events:           caseEventResponses(events),
		Evidence:         caseEvidenceResponses(evidence, attachments, false),
		Notification:     caseNotificationResponse(notification, false),
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
	if err := s.audit(ctx, guildContext, "case.history.read", "member", targetDiscordUserID, model.AuditResultSuccess, ""); err != nil {
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
	caseNumber := strings.TrimSpace(input.CaseNumber)
	if caseNumber != "" {
		parsed, parseErr := strconv.ParseUint(caseNumber, 10, 64)
		if parseErr != nil || parsed == 0 {
			return model.ListCasesParams{}, 0, 0, validationCaseError("case_number is invalid")
		}
	}
	actionResult := strings.TrimSpace(input.ActionResult)
	if actionResult != "" && !validActionExecutionStatus(model.ActionExecutionStatus(actionResult)) {
		return model.ListCasesParams{}, 0, 0, validationCaseError("action_result is invalid")
	}
	appealStatus := strings.TrimSpace(input.AppealStatus)
	if appealStatus != "" && !validAppealStatus(model.AppealStatus(appealStatus)) {
		return model.ListCasesParams{}, 0, 0, validationCaseError("appeal_status is invalid")
	}
	createdAfter, err := normalizeOptionalTime(input.CreatedAfter)
	if err != nil {
		return model.ListCasesParams{}, 0, 0, err
	}
	createdBefore, err := normalizeOptionalTime(input.CreatedBefore)
	if err != nil {
		return model.ListCasesParams{}, 0, 0, err
	}

	return model.ListCasesParams{
		GuildID:                guildContext.Guild.ID,
		TargetDiscordUserID:    strings.TrimSpace(input.TargetDiscordUserID),
		ModeratorDiscordUserID: strings.TrimSpace(input.ModeratorDiscordUserID),
		TemplateID:             strings.TrimSpace(input.TemplateID),
		Validity:               validity,
		CaseNumber:             caseNumber, ActionResult: actionResult, AppealStatus: appealStatus, CreatedAfter: createdAfter, CreatedBefore: createdBefore,
		Limit:  limit,
		Offset: offset,
	}, limit, offset, nil
}

// normalizeOptionalTime validates stable RFC3339 staff-search boundaries.
func normalizeOptionalTime(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", validationCaseError("date filter must use RFC3339")
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

// validActionExecutionStatus reports whether a staff action-result filter is supported.
func validActionExecutionStatus(value model.ActionExecutionStatus) bool {
	switch value {
	case model.ActionExecutionPending, model.ActionExecutionRunning, model.ActionExecutionSucceeded, model.ActionExecutionFailed, model.ActionExecutionRetrying, model.ActionExecutionSkipped, model.ActionExecutionCancelled:
		return true
	default:
		return false
	}
}

// validAppealStatus reports whether a staff appeal-status filter is supported.
func validAppealStatus(value model.AppealStatus) bool {
	switch value {
	case model.AppealStatusPending, model.AppealStatusAccepted, model.AppealStatusRejected, model.AppealStatusClosed:
		return true
	default:
		return false
	}
}

// Void preserves the case and correction reason while removing it from future escalation.
func (s *CaseService) Void(ctx context.Context, guildContext *GuildStaffContext, caseRef, reason string, replacementCaseID *string) (response *CaseResponse, err error) {
	defer func() {
		if err == nil || s == nil || s.store == nil || guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
			return
		}
		result := model.AuditResultFailure
		if errors.Is(err, ErrCasePermissionDenied) || errors.Is(err, ErrAuthorizationDenied) {
			result = model.AuditResultDenied
		}
		_ = s.audit(ctx, guildContext, string(model.AuditActionCaseVoid), "case", strings.TrimSpace(caseRef), result, err.Error())
	}()
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, validationCaseError("missing guild context")
	}
	if !guildContext.Can(model.PermissionActionCaseVoid) {
		return nil, ErrCasePermissionDenied
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, validationCaseError("void reason is required")
	}
	if replacementCaseID != nil {
		return nil, validationCaseError("create the replacement after voiding this case")
	}
	item, err := s.store.GetCaseByIDOrNumber(ctx, guildContext.Guild.ID, strings.TrimSpace(caseRef))
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCaseNotFound
	}
	voided, err := s.store.VoidCase(ctx, model.VoidCaseParams{GuildID: guildContext.Guild.ID, CaseID: item.ID, ActorDiscordUserID: guildContext.Staff.DiscordUserID, Reason: reason, ReplacementCaseID: replacementCaseID, Audit: s.auditEntry(ctx, guildContext, "case.void", "case", item.ID, model.AuditResultSuccess, "")})
	if err != nil {
		return nil, err
	}
	if voided == nil {
		return nil, ErrCaseNotFound
	}
	actions, err := s.store.ListCaseActionExecutions(ctx, voided.ID)
	if err != nil {
		return nil, err
	}
	result := caseResponseFromModel(*voided, actions)
	return &result, nil
}

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
}

// MemberEnforcementOutcome exposes only the configured action and public result.
type MemberEnforcementOutcome struct {
	ActionType model.ActionType            `json:"action_type"`
	Status     model.ActionExecutionStatus `json:"status"`
}

// ListMemberCases returns only cases targeting the authenticated Discord identity and does not require current guild membership.
func (s *CaseService) ListMemberCases(ctx context.Context, guildID, memberDiscordUserID string, input CaseListInput) (*CaseListResponse, error) {
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
	responses := make([]CaseResponse, 0, len(result.Cases))
	for _, item := range result.Cases {
		response := caseResponseFromModel(item, nil)
		response.ModeratorDiscordUserID = ""
		response.Metadata = nil
		response.Actions = nil
		responses = append(responses, response)
	}
	if err := s.memberReadAudit(ctx, guildID, memberDiscordUserID, "member_case.list", "guild", guildID); err != nil {
		return nil, err
	}
	return &CaseListResponse{Cases: responses, Total: result.Total, Limit: limit, Offset: offset}, nil
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
		_ = s.store.CreateAuditLogEntry(ctx, &model.AuditLogEntry{GuildID: item.GuildID, ActorDiscordUserID: strings.TrimSpace(memberDiscordUserID), Source: model.AuditSourceWeb, Action: "member_case.read", ResourceType: "case", ResourceID: item.ID, Result: model.AuditResultDenied, FailureReason: "not_case_target", RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"})
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
	return &MemberCaseDetail{ID: item.ID, GuildID: item.GuildID, CaseNumber: item.CaseNumber, TemplateID: item.TemplateID, Reason: item.Reason, Validity: item.Validity, VoidedReason: item.VoidedReason, ReplacementCaseID: item.ReplacementCaseID, CreatedAt: item.CreatedAt, ContextValues: parseCaseContextValues(item.ContextValuesJSON), SelectedLevel: selectedLevelResponse(item.TemplateSnapshotJSON), Enforcement: enforcement, Evidence: caseEvidenceResponses(evidence, attachments, true), Events: caseEventResponses(publicEvents), Notification: caseNotificationResponse(notification, true), Appealable: appealable}, nil
}

// memberReadAudit records target-owned reads without requiring a current staff or guild membership cache.
func (s *CaseService) memberReadAudit(ctx context.Context, guildID, actorID, action, resourceType, resourceID string) error {
	requestID, correlationID := TraceIDsFromContext(ctx)
	return s.store.CreateAuditLogEntry(ctx, &model.AuditLogEntry{GuildID: guildID, ActorDiscordUserID: actorID, Source: model.AuditSourceWeb, Action: action, ResourceType: resourceType, ResourceID: resourceID, Result: model.AuditResultSuccess, RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"})
}

// requireCaseRead encapsulates the require case read rule so callers share one consistent package implementation.
func (s *CaseService) requireCaseRead(guildContext *GuildStaffContext) error {
	if s == nil || s.store == nil {
		return errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return validationCaseError("missing guild context")
	}
	if !guildContext.Can(model.PermissionActionCaseRead) {
		return ErrCasePermissionDenied
	}
	return nil
}

// create validates and materializes a case within an already locked transaction, including the selected escalation level, immutable template snapshot, initial event, actions, and audit entry.
func (s *CaseService) create(ctx context.Context, guildContext *GuildStaffContext, input CaseInput, preflight *caseCreatePreflight, attribution caseCreateAttribution) (*model.CreatedCase, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("case service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, validationCaseError("missing guild context")
	}
	if !guildContext.Can(model.PermissionActionCaseCreate) {
		return nil, ErrCasePermissionDenied
	}
	if input.IdempotencyKey != "" {
		existing, err := s.store.GetCaseByIdempotencyKey(ctx, guildContext.Guild.ID, input.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			actions, actionErr := s.store.ListCaseActionExecutions(ctx, existing.ID)
			if actionErr != nil {
				return nil, actionErr
			}
			return &model.CreatedCase{Case: *existing, ActionExecutions: actions}, nil
		}
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
	if preflight == nil {
		return nil, validationCaseError("case preflight is required")
	}
	actualAction := model.ActionType("")
	if len(selectedLevel.Actions) == 1 {
		actualAction = selectedLevel.Actions[0].ActionType
	}
	if preflight.TemplateID != template.Template.ID || preflight.TemplateVersion != template.Template.Version || preflight.SelectedLevelID != selectedLevel.Level.ID || preflight.ActionType != actualAction {
		return nil, errCasePreflightStale
	}
	contextValuesJSON := preflight.ContextValuesJSON
	captured := preflight.Captured

	snapshotJSON, err := buildTemplateSnapshot(template.Template, template.ContextFields, contextValuesJSON, *selectedLevel)
	if err != nil {
		return nil, err
	}
	_, correlationID := TraceIDsFromContext(ctx)

	actorDiscordUserID := guildContext.Staff.DiscordUserID
	if attribution.system {
		actorDiscordUserID = ""
	}
	caseModel := model.Case{
		GuildID:                 guildContext.Guild.ID,
		TemplateID:              &template.Template.ID,
		TemplateVersion:         template.Template.Version,
		TemplateSnapshotJSON:    snapshotJSON,
		TargetDiscordUserID:     targetDiscordUserID,
		ModeratorDiscordUserID:  actorDiscordUserID,
		Reason:                  reason,
		Validity:                model.CaseValidityValid,
		Source:                  source,
		CorrelationID:           correlationID,
		ContextChannelDiscordID: strings.TrimSpace(input.ContextChannelDiscordID),
		ContextMessageDiscordID: strings.TrimSpace(input.ContextMessageDiscordID),
		ContextURL:              strings.TrimSpace(input.ContextURL),
		MetadataJSON:            metadataJSON,
		ContextValuesJSON:       contextValuesJSON,
	}
	if input.IdempotencyKey != "" {
		key := input.IdempotencyKey
		caseModel.IdempotencyKey = &key
	}
	if replacement := strings.TrimSpace(input.ReplacesCaseID); replacement != "" {
		prior, getErr := s.store.GetCaseByIDOrNumber(ctx, guildContext.Guild.ID, replacement)
		if getErr != nil {
			return nil, getErr
		}
		if prior == nil || prior.Validity != model.CaseValidityVoided {
			return nil, validationCaseError("replacement must reference a voided case in this guild")
		}
		caseModel.ReplacesCaseID = &prior.ID
	}

	actionExecutions := make([]model.CaseActionExecution, 0, len(selectedLevel.Actions))
	for _, action := range selectedLevel.Actions {
		templateActionID := action.ID
		actionExecutions = append(actionExecutions, model.CaseActionExecution{
			TemplateActionID:   &templateActionID,
			Position:           0,
			ActionType:         action.ActionType,
			Status:             model.ActionExecutionPending,
			ConfigSnapshotJSON: action.ConfigJSON,
			MaxRetries:         action.MaxRetries,
			RetryBackoffMS:     1000,
			SafeForRetry:       true,
			Irreversible:       irreversibleAction(action.ActionType),
			CorrelationID:      correlationID,
		})
	}
	var notification *model.CaseNotification
	if selectedLevel.Level.NotifyUser {
		notification = &model.CaseNotification{Status: model.NotificationPending}
	}

	event := model.CaseEvent{
		EventType:          model.CaseEventCreated,
		ActorDiscordUserID: actorDiscordUserID,
		ActorType:          attribution.actorType,
		Visibility:         model.EventVisibilityPublic,
		Body:               fmt.Sprintf("Case created from template %s", template.Template.Slug),
		MetadataJSON:       "{}",
	}

	params := model.CreateCaseParams{
		Case:             caseModel,
		Event:            event,
		ActionExecutions: actionExecutions,
		Evidence:         captured.Snapshots,
		Attachments:      captured.Attachments,
		Notification:     notification,
		Audit:            s.auditEntryWithAttribution(ctx, guildContext, attribution, "case.create", "case", "", model.AuditResultSuccess, ""),
	}
	if len(captured.Snapshots) > 0 {
		result := model.AuditResultSuccess
		failure := ""
		if len(captured.Warnings) > 0 {
			result = model.AuditResultFailure
			failure = "partial evidence capture"
		}
		entry := s.auditEntryWithAttribution(ctx, guildContext, attribution, "evidence.capture", "case_evidence", "", result, failure)
		if entry != nil {
			entry.MetadataJSON = mustMarshalJSONObject(map[string]any{"snapshot_count": len(captured.Snapshots), "attachment_count": len(captured.Attachments), "partial": len(captured.Warnings) > 0})
			params.AdditionalAudits = append(params.AdditionalAudits, *entry)
		}
	}
	return s.store.CreateCase(ctx, params)
}

// selectTemplateLevel chooses the highest escalation whose all-time historical-case threshold is met, falling back to the default level.
func (s *CaseService) selectTemplateLevel(ctx context.Context, guildID, targetDiscordUserID string, template *model.ExpandedCaseTemplate) (*selectedTemplateLevel, error) {
	if template == nil {
		return nil, validationCaseError("template is required")
	}

	var fallback *selectedTemplateLevel
	var best *selectedTemplateLevel
	matchedCaseCount, err := s.matchingTemplateCaseCount(ctx, guildID, targetDiscordUserID, template.Template.ID)
	if err != nil {
		return nil, err
	}
	for _, expandedLevel := range template.Levels {
		level := expandedLevel.Level
		if len(expandedLevel.Actions) > 1 {
			return nil, validationCaseError("template level has more than one enforcement action")
		}
		if level.IsDefault {
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
func buildTemplateSnapshot(template model.CaseTemplate, fields []model.CaseTemplateContextField, valuesJSON string, selectedLevel selectedTemplateLevel) (string, error) {
	snapshot := templateSnapshot{
		Template: templateSnapshotTemplate{
			ID:             template.ID,
			Slug:           template.Slug,
			Name:           template.Name,
			Version:        template.Version,
			ReasonTemplate: template.ReasonTemplate,
			Appealable:     template.Appealable,
		},
		SelectedLevel: CaseSelectedLevel{
			TemplateLevelDetails: templateLevelDetails(selectedLevel.Level),
			MatchedCaseCount:     selectedLevel.MatchedCaseCount,
		},
		Actions:       make([]templateSnapshotAction, 0, len(selectedLevel.Actions)),
		ContextFields: make([]TemplateContextFieldResponse, 0, len(fields)),
	}
	_ = json.Unmarshal([]byte(valuesJSON), &snapshot.ContextValues)
	for _, field := range fields {
		snapshot.ContextFields = append(snapshot.ContextFields, TemplateContextFieldResponse{ID: field.ID, Key: field.Key, Label: field.Label, FieldType: field.FieldType, Position: field.Position, Required: field.Required})
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
		Metadata:                parseJSON(caseModel.MetadataJSON), ContextValues: parseCaseContextValues(caseModel.ContextValuesJSON), VoidedReason: caseModel.VoidedReason, VoidedAt: caseModel.VoidedAt, ReplacementCaseID: caseModel.ReplacementCaseID, ReplacesCaseID: caseModel.ReplacesCaseID,
		SelectedLevel: selectedLevelResponse(caseModel.TemplateSnapshotJSON),
		Actions:       make([]CaseActionResponse, 0, len(actionExecutions)),
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
		Template      templateSnapshotTemplate       `json:"template"`
		SelectedLevel CaseSelectedLevel              `json:"selected_level"`
		Actions       []json.RawMessage              `json:"actions"`
		ContextFields []TemplateContextFieldResponse `json:"context_fields"`
		ContextValues []CaseContextValueResponse     `json:"context_values"`
	}
	if err := json.Unmarshal([]byte(snapshotJSON), &stored); err != nil || stored.Template.ID == "" {
		return nil
	}
	snapshot := CaseTemplateSnapshotResponse{
		Template:      stored.Template,
		SelectedLevel: stored.SelectedLevel,
		Actions:       make([]templateSnapshotAction, 0, len(stored.Actions)),
		ContextFields: stored.ContextFields, ContextValues: stored.ContextValues,
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

// validateCaseContextValues binds submitted values to the immutable template definitions and returns message links for shared capture.
func validateCaseContextValues(fields []model.CaseTemplateContextField, inputs []CaseContextValueInput) (string, []string, bool, error) {
	byKey := make(map[string]json.RawMessage, len(inputs))
	for _, input := range inputs {
		key := strings.ToLower(strings.TrimSpace(input.Key))
		if key == "" {
			return "", nil, false, validationCaseError("context value key is required")
		}
		if _, duplicate := byKey[key]; duplicate {
			return "", nil, false, validationCaseError("duplicate context value")
		}
		byKey[key] = input.Value
	}
	values := make([]CaseContextValueResponse, 0, len(fields))
	links := []string{}
	hasFallback := false
	for _, field := range fields {
		raw, provided := byKey[field.Key]
		if !provided || len(raw) == 0 || string(raw) == "null" {
			if field.Required {
				return "", nil, false, validationCaseError("required context value is missing: " + field.Key)
			}
			values = append(values, CaseContextValueResponse{Key: field.Key, Label: field.Label, FieldType: field.FieldType, Required: field.Required, Value: nil})
			delete(byKey, field.Key)
			continue
		}
		var value any
		switch field.FieldType {
		case model.ContextFieldShortText, model.ContextFieldLongText, model.ContextFieldMessageLink:
			var text string
			if json.Unmarshal(raw, &text) != nil {
				return "", nil, false, validationCaseError("context value has wrong type: " + field.Key)
			}
			text = strings.TrimSpace(text)
			limit := 4000
			if field.FieldType == model.ContextFieldShortText {
				limit = 500
			}
			if text == "" && field.Required {
				return "", nil, false, validationCaseError("required context value is empty: " + field.Key)
			}
			if len([]rune(text)) > limit {
				return "", nil, false, validationCaseError("context value is too long: " + field.Key)
			}
			value = text
			if field.FieldType == model.ContextFieldMessageLink && text != "" {
				links = append(links, text)
			} else if text != "" {
				hasFallback = true
			}
		case model.ContextFieldBoolean:
			var boolean bool
			if json.Unmarshal(raw, &boolean) != nil {
				return "", nil, false, validationCaseError("context value has wrong type: " + field.Key)
			}
			value = boolean
			hasFallback = true
		case model.ContextFieldNumber:
			var number json.Number
			decoder := json.NewDecoder(strings.NewReader(string(raw)))
			decoder.UseNumber()
			if decoder.Decode(&number) != nil {
				return "", nil, false, validationCaseError("context value has wrong type: " + field.Key)
			}
			if _, err := number.Float64(); err != nil {
				return "", nil, false, validationCaseError("context number is invalid: " + field.Key)
			}
			value = number
			hasFallback = true
		default:
			return "", nil, false, validationCaseError("context field type is invalid")
		}
		values = append(values, CaseContextValueResponse{Key: field.Key, Label: field.Label, FieldType: field.FieldType, Required: field.Required, Value: value})
		delete(byKey, field.Key)
	}
	if len(byKey) > 0 {
		return "", nil, false, validationCaseError("unknown context value")
	}
	body, err := json.Marshal(values)
	if err != nil {
		return "", nil, false, err
	}
	return string(body), links, hasFallback, nil
}

// parseCaseContextValues safely decodes the immutable member-visible context snapshot.
func parseCaseContextValues(body string) []CaseContextValueResponse {
	var values []CaseContextValueResponse
	if json.Unmarshal([]byte(body), &values) != nil {
		return []CaseContextValueResponse{}
	}
	return values
}

// caseEvidenceResponses applies staff or member evidence projection rules.
func caseEvidenceResponses(snapshots []model.CaseEvidenceSnapshot, attachments []model.CaseEvidenceAttachment, member bool) []CaseEvidenceResponse {
	byEvidence := map[string][]CaseEvidenceAttachmentResponse{}
	for _, item := range attachments {
		original := item.OriginalURL
		if member && item.PreservedURL != "" {
			original = ""
		}
		byEvidence[item.EvidenceID] = append(byEvidence[item.EvidenceID], CaseEvidenceAttachmentResponse{Filename: item.Filename, ContentType: item.ContentType, SizeBytes: item.SizeBytes, OriginalURL: original, PreservedURL: item.PreservedURL, CopyOutcome: item.CopyOutcome, Warning: item.Warning})
	}
	out := make([]CaseEvidenceResponse, 0, len(snapshots))
	for _, item := range snapshots {
		out = append(out, CaseEvidenceResponse{ID: item.ID, AuthorDiscordUserID: item.AuthorDiscordUserID, MessageURL: item.MessageURL, Content: item.Content, MessageCreatedAt: item.MessageCreatedAt, MessageEditedAt: item.MessageEditedAt, Embeds: parseJSON(item.EmbedsJSON), CaptureOutcome: item.CaptureOutcome, CaptureWarning: item.CaptureWarning, Attachments: byEvidence[item.ID]})
	}
	return out
}

// caseNotificationResponse hides internal delivery diagnostics from affected members.
func caseNotificationResponse(item *model.CaseNotification, member bool) *CaseNotificationResponse {
	if item == nil {
		return nil
	}
	response := &CaseNotificationResponse{Status: item.Status, AttemptCount: item.AttemptCount, LastErrorCode: item.LastErrorCode, LastError: item.LastError, SentAt: item.SentAt}
	if member {
		response.LastErrorCode = ""
		response.LastError = ""
	}
	return response
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
	return s.auditWithAttribution(ctx, guildContext, caseCreateAttribution{actorType: "staff", auditSource: model.AuditSourceAPI}, action, resourceType, resourceID, result, failureReason)
}

// auditWithAttribution appends case evidence without inventing a Discord actor
// for system automation.
func (s *CaseService) auditWithAttribution(ctx context.Context, guildContext *GuildStaffContext, attribution caseCreateAttribution, action, resourceType, resourceID string, result model.AuditResult, failureReason string) error {
	entry := s.auditEntryWithAttribution(ctx, guildContext, attribution, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return s.store.CreateAuditLogEntry(ctx, entry)
}

// auditEntry records audit entry so moderation changes remain attributable.
func (s *CaseService) auditEntry(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) *model.AuditLogEntry {
	return s.auditEntryWithAttribution(ctx, guildContext, caseCreateAttribution{actorType: "staff", auditSource: model.AuditSourceAPI}, action, resourceType, resourceID, result, failureReason)
}

// auditEntryWithAttribution builds the atomic case audit row for either a
// current staff actor or Quack's restricted honeypot system actor.
func (s *CaseService) auditEntryWithAttribution(ctx context.Context, guildContext *GuildStaffContext, attribution caseCreateAttribution, action, resourceType, resourceID string, result model.AuditResult, failureReason string) *model.AuditLogEntry {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil
	}
	requestID, correlationID := TraceIDsFromContext(ctx)
	actorDiscordUserID := guildContext.Staff.DiscordUserID
	permissionBits := guildContext.PermissionBits
	if attribution.system {
		actorDiscordUserID = ""
		permissionBits = 0
	}

	entry := &model.AuditLogEntry{
		GuildID:             guildContext.Guild.ID,
		ActorDiscordUserID:  actorDiscordUserID,
		ActorPermissionBits: permissionBits,
		Source:              AuditSourceFromContext(ctx),
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
