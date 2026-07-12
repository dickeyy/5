package quack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
)

var (
	// ErrAppealNotFound prevents unrelated members from distinguishing another member's appeal.
	ErrAppealNotFound = errors.New("appeal not found")
	// ErrAppealValidation reports a malformed form, answer, transition, or reason.
	ErrAppealValidation = errors.New("appeal validation failed")
	// ErrAppealPermissionDenied reports missing live staff review authority.
	ErrAppealPermissionDenied = errors.New("appeal permission denied")
	// ErrAppealConflict reports a duplicate submission or stale timeline transition.
	ErrAppealConflict = errors.New("appeal state conflict")
)

// AppealRepository is the package-owned persistence boundary exposed by the store adapter.
type AppealRepository interface {
	GetGuildAppealSettings(context.Context, string) (*model.GuildAppealSettings, error)
	UpdateGuildAppealSettings(context.Context, model.UpdateGuildAppealSettingsParams) (*model.GuildAppealSettings, error)
	CreateAppeal(context.Context, model.CreateAppealParams) (*model.Appeal, error)
	GetAppealByID(context.Context, string) (*model.Appeal, error)
	GetAppealByCaseID(context.Context, string) (*model.Appeal, error)
	ListAppeals(context.Context, model.AppealListParams) (*model.AppealListResult, error)
	ListAppealEvents(context.Context, string) ([]model.AppealEvent, error)
	AppendAppealInformation(context.Context, model.AppendAppealInformationParams) (*model.Appeal, error)
	TransitionAppeal(context.Context, model.TransitionAppealParams) (*model.Appeal, error)
	ClaimPendingAppealNotifications(context.Context, int) ([]model.AppealNotification, error)
	CompleteAppealNotification(context.Context, model.CompleteAppealNotificationParams) error
	GetCaseByID(context.Context, string) (*model.Case, error)
	ListCaseActionExecutions(context.Context, string) ([]model.CaseActionExecution, error)
	CreateAuditLogEntry(context.Context, *model.AuditLogEntry) error
}

// AppealService owns case eligibility, form snapshots, ownership, state transitions, and privacy projections.
type AppealService struct {
	store AppealRepository
}

// AppealSettingsResponse returns the effective future form, including Quack's default when no override exists.
type AppealSettingsResponse struct {
	GuildID   string                 `json:"guild_id"`
	Questions []model.AppealQuestion `json:"questions"`
	Default   bool                   `json:"default"`
}

// AppealSubmissionInput carries answers to the effective snapshotted form.
type AppealSubmissionInput struct {
	Answers []model.AppealAnswer `json:"answers"`
}

// AppealInformationInput carries a member's immutable response to a staff request.
type AppealInformationInput struct {
	Body string `json:"body"`
}

// AppealDecisionInput carries the required public-safe reason for one staff transition.
type AppealDecisionInput struct {
	Reason string `json:"reason"`
}

// AppealEventResponse is one timeline entry; member projections omit staff identity.
type AppealEventResponse struct {
	ID                 string                `json:"id"`
	Type               model.AppealEventType `json:"type"`
	ActorType          string                `json:"actor_type"`
	ActorDiscordUserID string                `json:"actor_discord_user_id,omitempty"`
	Body               string                `json:"body"`
	CreatedAt          time.Time             `json:"created_at"`
}

// AppealReversalOffer describes a separately confirmed reversal without executing it.
type AppealReversalOffer struct {
	OriginalExecutionID string           `json:"original_execution_id"`
	ActionType          model.ActionType `json:"action_type"`
}

// AppealResponse is the complete case-linked appeal projection.
type AppealResponse struct {
	ID                      string                 `json:"id"`
	GuildID                 string                 `json:"guild_id"`
	CaseID                  string                 `json:"case_id"`
	TargetDiscordUserID     string                 `json:"target_discord_user_id"`
	Status                  model.AppealStatus     `json:"status"`
	Questions               []model.AppealQuestion `json:"questions"`
	Answers                 []model.AppealAnswer   `json:"answers"`
	DecisionReason          string                 `json:"decision_reason,omitempty"`
	ReviewedByDiscordUserID string                 `json:"reviewed_by_discord_user_id,omitempty"`
	Events                  []AppealEventResponse  `json:"events"`
	ReversalOffers          []AppealReversalOffer  `json:"reversal_offers,omitempty"`
	CreatedAt               time.Time              `json:"created_at"`
	UpdatedAt               time.Time              `json:"updated_at"`
}

// AppealListResponse returns stable staff queue pagination.
type AppealListResponse struct {
	Appeals []AppealResponse `json:"appeals"`
	Total   int64            `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
}

// NewAppealService constructs the package service without central runtime ownership.
func NewAppealService(store AppealRepository) *AppealService {
	return &AppealService{store: store}
}

// DefaultAppealQuestions returns Quack's stable, simple default form.
func DefaultAppealQuestions() []model.AppealQuestion {
	return []model.AppealQuestion{
		{ID: "reason", Prompt: "Why should this case be reconsidered?", Type: model.AppealQuestionLongText, Required: true, Position: 0},
		{ID: "context", Prompt: "Is there any additional context staff should review?", Type: model.AppealQuestionLongText, Required: false, Position: 1},
	}
}

// GetSettings returns the configured or default appeal form for one guild.
func (s *AppealService) GetSettings(ctx context.Context, guildID string) (*AppealSettingsResponse, error) {
	if s == nil || s.store == nil || strings.TrimSpace(guildID) == "" {
		return nil, appealValidation("guild is required")
	}
	settings, err := s.store.GetGuildAppealSettings(ctx, strings.TrimSpace(guildID))
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return &AppealSettingsResponse{GuildID: guildID, Questions: DefaultAppealQuestions(), Default: true}, nil
	}
	questions, err := decodeQuestions(settings.QuestionsJSON)
	if err != nil {
		return nil, err
	}
	return &AppealSettingsResponse{GuildID: settings.GuildID, Questions: questions}, nil
}

// UpdateSettings validates and replaces only the form snapshotted by future appeals.
func (s *AppealService) UpdateSettings(ctx context.Context, guildContext *GuildStaffContext, questions []model.AppealQuestion) (*AppealSettingsResponse, error) {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil || !guildContext.Can(model.PermissionActionGuildSettingsWrite) {
		return nil, ErrAppealPermissionDenied
	}
	normalized, err := validateQuestions(questions)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(normalized)
	settings, err := s.store.UpdateGuildAppealSettings(ctx, model.UpdateGuildAppealSettingsParams{
		Settings: model.GuildAppealSettings{GuildID: guildContext.Guild.ID, QuestionsJSON: string(body), UpdatedByDiscordUserID: guildContext.Staff.DiscordUserID},
		Audit:    appealAudit(ctx, guildContext.Guild.ID, guildContext.Staff.DiscordUserID, guildContext.PermissionBits, "appeal.settings.update", "guild_appeal_settings", "", model.AuditResultSuccess),
	})
	if err != nil {
		return nil, err
	}
	return &AppealSettingsResponse{GuildID: settings.GuildID, Questions: normalized}, nil
}

// Submit creates the only appeal for an eligible case owned by the authenticated identity.
func (s *AppealService) Submit(ctx context.Context, caseID, memberDiscordUserID string, input AppealSubmissionInput) (*AppealResponse, error) {
	caseID = strings.TrimSpace(caseID)
	memberDiscordUserID = strings.TrimSpace(memberDiscordUserID)
	if caseID == "" || memberDiscordUserID == "" {
		return nil, appealValidation("case and member identity are required")
	}
	item, err := s.store.GetCaseByID(ctx, caseID)
	if err != nil {
		return nil, err
	}
	if item == nil || item.TargetDiscordUserID != memberDiscordUserID {
		if item != nil {
			_ = s.auditMember(ctx, item.GuildID, memberDiscordUserID, "appeal.submit", item.ID, model.AuditResultDenied)
		}
		return nil, ErrAppealNotFound
	}
	if item.Validity != model.CaseValidityValid || !caseSnapshotAppealable(item.TemplateSnapshotJSON) {
		_ = s.auditMember(ctx, item.GuildID, memberDiscordUserID, "appeal.submit", item.ID, model.AuditResultDenied)
		return nil, model.ErrAppealCaseIneligible
	}
	if existing, getErr := s.store.GetAppealByCaseID(ctx, item.ID); getErr != nil {
		return nil, getErr
	} else if existing != nil {
		return nil, ErrAppealConflict
	}
	settings, err := s.GetSettings(ctx, item.GuildID)
	if err != nil {
		return nil, err
	}
	answers, err := validateAnswers(settings.Questions, input.Answers)
	if err != nil {
		return nil, err
	}
	questionJSON, _ := json.Marshal(settings.Questions)
	answersJSON, _ := json.Marshal(answers)
	caseIDCopy := item.ID
	appeal := model.Appeal{GuildID: item.GuildID, CaseID: &caseIDCopy, TargetDiscordUserID: memberDiscordUserID, Status: model.AppealStatusPending, QuestionSnapshotJSON: string(questionJSON), AnswersJSON: string(answersJSON), Version: 1, MetadataJSON: "{}"}
	created, err := s.store.CreateAppeal(ctx, model.CreateAppealParams{
		Appeal:       appeal,
		Event:        model.AppealEvent{EventType: string(model.AppealEventSubmitted), ActorDiscordUserID: memberDiscordUserID, ActorType: "member", Body: "Appeal submitted", MetadataJSON: "{}"},
		CaseEvent:    model.CaseEvent{EventType: model.CaseEventAppealCreated, ActorDiscordUserID: memberDiscordUserID, ActorType: "member", Visibility: model.EventVisibilityPublic, Body: "Appeal submitted", MetadataJSON: "{}"},
		Audit:        appealAudit(ctx, item.GuildID, memberDiscordUserID, 0, "appeal.submit", "appeal", "", model.AuditResultSuccess),
		Notification: model.AppealNotification{TargetDiscordUserID: memberDiscordUserID, Audience: model.AppealNotificationStaff, Status: model.AppealNotificationPending, Body: fmt.Sprintf("A new appeal was submitted for case #%d.", item.CaseNumber)},
	})
	if errors.Is(err, model.ErrAppealAlreadyExists) {
		return nil, ErrAppealConflict
	}
	if err != nil {
		return nil, err
	}
	return s.response(ctx, created, true)
}

// GetMember returns an appeal only to its target identity and redacts every staff actor.
func (s *AppealService) GetMember(ctx context.Context, appealID, memberDiscordUserID string) (*AppealResponse, error) {
	item, err := s.store.GetAppealByID(ctx, strings.TrimSpace(appealID))
	if err != nil {
		return nil, err
	}
	if item == nil || item.TargetDiscordUserID != strings.TrimSpace(memberDiscordUserID) {
		if item != nil {
			_ = s.auditMember(ctx, item.GuildID, memberDiscordUserID, "appeal.read", item.ID, model.AuditResultDenied)
		}
		return nil, ErrAppealNotFound
	}
	if err := s.auditMember(ctx, item.GuildID, memberDiscordUserID, "appeal.read", item.ID, model.AuditResultSuccess); err != nil {
		return nil, err
	}
	return s.response(ctx, item, true)
}

// SubmitInformation appends member information to the existing timeline and returns it to staff review.
func (s *AppealService) SubmitInformation(ctx context.Context, appealID, memberDiscordUserID string, input AppealInformationInput) (*AppealResponse, error) {
	body := strings.TrimSpace(input.Body)
	if body == "" || len([]rune(body)) > 4000 {
		return nil, appealValidation("information must be between 1 and 4000 characters")
	}
	item, err := s.store.GetAppealByID(ctx, strings.TrimSpace(appealID))
	if err != nil {
		return nil, err
	}
	if item == nil || item.TargetDiscordUserID != strings.TrimSpace(memberDiscordUserID) {
		return nil, ErrAppealNotFound
	}
	updated, err := s.store.AppendAppealInformation(ctx, model.AppendAppealInformationParams{
		AppealID: item.ID, TargetDiscordUserID: memberDiscordUserID, Body: body,
		Event:        model.AppealEvent{EventType: string(model.AppealEventInformationAdded), ActorDiscordUserID: memberDiscordUserID, ActorType: "member", Body: body, MetadataJSON: "{}"},
		Audit:        appealAudit(ctx, item.GuildID, memberDiscordUserID, 0, "appeal.information.submit", "appeal", item.ID, model.AuditResultSuccess),
		Notification: model.AppealNotification{TargetDiscordUserID: memberDiscordUserID, Audience: model.AppealNotificationStaff, Status: model.AppealNotificationPending, Body: "A member submitted additional appeal information."},
	})
	if errors.Is(err, model.ErrAppealStateConflict) {
		return nil, ErrAppealConflict
	}
	if err != nil {
		return nil, err
	}
	return s.response(ctx, updated, true)
}

// GetStaff returns the exact staff projection after live Moderate Members authorization.
func (s *AppealService) GetStaff(ctx context.Context, guildContext *GuildStaffContext, appealID string) (*AppealResponse, error) {
	if err := requireAppealReview(guildContext); err != nil {
		return nil, err
	}
	item, err := s.store.GetAppealByID(ctx, strings.TrimSpace(appealID))
	if err != nil {
		return nil, err
	}
	if item == nil || item.GuildID != guildContext.Guild.ID {
		return nil, ErrAppealNotFound
	}
	if err := s.store.CreateAuditLogEntry(ctx, pointerAudit(appealAudit(ctx, item.GuildID, guildContext.Staff.DiscordUserID, guildContext.PermissionBits, "appeal.read", "appeal", item.ID, model.AuditResultSuccess))); err != nil {
		return nil, err
	}
	return s.response(ctx, item, false)
}

// ListStaff returns the authorized guild queue with stable pagination and optional state filter.
func (s *AppealService) ListStaff(ctx context.Context, guildContext *GuildStaffContext, status model.AppealStatus, limit, offset int) (*AppealListResponse, error) {
	if err := requireAppealReview(guildContext); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 || offset < 0 || (status != "" && !validAppealState(status)) {
		return nil, appealValidation("invalid appeal queue filter")
	}
	result, err := s.store.ListAppeals(ctx, model.AppealListParams{GuildID: guildContext.Guild.ID, Status: status, Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	responses := make([]AppealResponse, 0, len(result.Appeals))
	for index := range result.Appeals {
		response, responseErr := s.response(ctx, &result.Appeals[index], false)
		if responseErr != nil {
			return nil, responseErr
		}
		responses = append(responses, *response)
	}
	if err := s.store.CreateAuditLogEntry(ctx, pointerAudit(appealAudit(ctx, guildContext.Guild.ID, guildContext.Staff.DiscordUserID, guildContext.PermissionBits, "appeal.queue.read", "appeal", "list", model.AuditResultSuccess))); err != nil {
		return nil, err
	}
	return &AppealListResponse{Appeals: responses, Total: result.Total, Limit: limit, Offset: offset}, nil
}

// RequestInformation asks the member for more information without exposing the staff actor.
func (s *AppealService) RequestInformation(ctx context.Context, guildContext *GuildStaffContext, appealID, reason string) (*AppealResponse, error) {
	return s.transition(ctx, guildContext, appealID, reason, []model.AppealStatus{model.AppealStatusPending}, model.AppealStatusNeedsInformation, model.AppealEventInformationAsked, false)
}

// Reopen reuses a rejected or closed appeal to request additional information rather than creating another record.
func (s *AppealService) Reopen(ctx context.Context, guildContext *GuildStaffContext, appealID, reason string) (*AppealResponse, error) {
	return s.transition(ctx, guildContext, appealID, reason, []model.AppealStatus{model.AppealStatusRejected, model.AppealStatusClosed}, model.AppealStatusNeedsInformation, model.AppealEventReopened, false)
}

// Accept records the decision and atomically voids the case; it never queues a Discord reversal.
func (s *AppealService) Accept(ctx context.Context, guildContext *GuildStaffContext, appealID, reason string) (*AppealResponse, error) {
	return s.transition(ctx, guildContext, appealID, reason, []model.AppealStatus{model.AppealStatusPending}, model.AppealStatusAccepted, model.AppealEventAccepted, true)
}

// Reject records a terminal decision without changing case validity.
func (s *AppealService) Reject(ctx context.Context, guildContext *GuildStaffContext, appealID, reason string) (*AppealResponse, error) {
	return s.transition(ctx, guildContext, appealID, reason, []model.AppealStatus{model.AppealStatusPending}, model.AppealStatusRejected, model.AppealEventRejected, false)
}

// Close ends an undecided or information-waiting appeal without changing case validity.
func (s *AppealService) Close(ctx context.Context, guildContext *GuildStaffContext, appealID, reason string) (*AppealResponse, error) {
	return s.transition(ctx, guildContext, appealID, reason, []model.AppealStatus{model.AppealStatusPending, model.AppealStatusNeedsInformation}, model.AppealStatusClosed, model.AppealEventClosed, false)
}

func (s *AppealService) transition(ctx context.Context, guildContext *GuildStaffContext, appealID, reason string, from []model.AppealStatus, to model.AppealStatus, eventType model.AppealEventType, voidCase bool) (*AppealResponse, error) {
	if err := requireAppealReview(guildContext); err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > 2000 {
		return nil, appealValidation("reason must be between 1 and 2000 characters")
	}
	item, err := s.store.GetAppealByID(ctx, strings.TrimSpace(appealID))
	if err != nil {
		return nil, err
	}
	if item == nil || item.GuildID != guildContext.Guild.ID {
		return nil, ErrAppealNotFound
	}
	params := model.TransitionAppealParams{
		GuildID: item.GuildID, AppealID: item.ID, ActorDiscordUserID: guildContext.Staff.DiscordUserID,
		AllowedFrom: from, To: to, Reason: reason, VoidCase: voidCase,
		Event:        model.AppealEvent{EventType: string(eventType), ActorDiscordUserID: guildContext.Staff.DiscordUserID, ActorType: "staff", Body: reason, MetadataJSON: "{}"},
		AppealAudit:  appealAudit(ctx, item.GuildID, guildContext.Staff.DiscordUserID, guildContext.PermissionBits, "appeal."+string(eventType), "appeal", item.ID, model.AuditResultSuccess),
		Notification: model.AppealNotification{TargetDiscordUserID: item.TargetDiscordUserID, Audience: model.AppealNotificationMember, Status: model.AppealNotificationPending, Body: memberNotificationBody(to, reason)},
	}
	if voidCase {
		caseAudit := appealAudit(ctx, item.GuildID, guildContext.Staff.DiscordUserID, guildContext.PermissionBits, "case.void.appeal", "case", "", model.AuditResultSuccess)
		params.CaseAudit = &caseAudit
	}
	updated, err := s.store.TransitionAppeal(ctx, params)
	if errors.Is(err, model.ErrAppealStateConflict) || errors.Is(err, model.ErrAppealCaseIneligible) {
		return nil, ErrAppealConflict
	}
	if err != nil {
		return nil, err
	}
	return s.response(ctx, updated, false)
}

func (s *AppealService) response(ctx context.Context, item *model.Appeal, member bool) (*AppealResponse, error) {
	questions, err := decodeQuestions(item.QuestionSnapshotJSON)
	if err != nil {
		return nil, err
	}
	var answers []model.AppealAnswer
	if err := json.Unmarshal([]byte(item.AnswersJSON), &answers); err != nil {
		return nil, fmt.Errorf("decode appeal answers: %w", err)
	}
	events, err := s.store.ListAppealEvents(ctx, item.ID)
	if err != nil {
		return nil, err
	}
	responseEvents := make([]AppealEventResponse, 0, len(events))
	for _, event := range events {
		actorID := event.ActorDiscordUserID
		if member && event.ActorType == "staff" {
			actorID = ""
		}
		responseEvents = append(responseEvents, AppealEventResponse{ID: event.ID, Type: model.AppealEventType(event.EventType), ActorType: event.ActorType, ActorDiscordUserID: actorID, Body: event.Body, CreatedAt: event.CreatedAt})
	}
	reviewedBy := item.ReviewedByDiscordUserID
	if member {
		reviewedBy = ""
	}
	caseID := ""
	if item.CaseID != nil {
		caseID = *item.CaseID
	}
	response := &AppealResponse{ID: item.ID, GuildID: item.GuildID, CaseID: caseID, TargetDiscordUserID: item.TargetDiscordUserID, Status: item.Status, Questions: questions, Answers: answers, DecisionReason: item.DecisionReason, ReviewedByDiscordUserID: reviewedBy, Events: responseEvents, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	if !member && item.Status == model.AppealStatusAccepted && caseID != "" {
		actions, actionErr := s.store.ListCaseActionExecutions(ctx, caseID)
		if actionErr != nil {
			return nil, actionErr
		}
		for _, action := range actions {
			if action.Status != model.ActionExecutionSucceeded || action.ReversalOfExecutionID != nil {
				continue
			}
			switch action.ActionType {
			case model.ActionTimeoutUser:
				response.ReversalOffers = append(response.ReversalOffers, AppealReversalOffer{OriginalExecutionID: action.ID, ActionType: model.ActionRemoveTimeout})
			case model.ActionBanUser:
				response.ReversalOffers = append(response.ReversalOffers, AppealReversalOffer{OriginalExecutionID: action.ID, ActionType: model.ActionUnbanUser})
			}
		}
	}
	return response, nil
}

func validateQuestions(questions []model.AppealQuestion) ([]model.AppealQuestion, error) {
	if len(questions) == 0 || len(questions) > 10 {
		return nil, appealValidation("appeal form must contain between 1 and 10 questions")
	}
	normalized := append([]model.AppealQuestion(nil), questions...)
	sort.SliceStable(normalized, func(i, j int) bool { return normalized[i].Position < normalized[j].Position })
	seen := map[string]bool{}
	for index := range normalized {
		question := &normalized[index]
		question.ID = strings.TrimSpace(question.ID)
		question.Prompt = strings.TrimSpace(question.Prompt)
		if question.ID == "" || len(question.ID) > 64 || question.Prompt == "" || len([]rune(question.Prompt)) > 300 || seen[question.ID] || question.Position != index {
			return nil, appealValidation("appeal questions require unique ids and contiguous ordering")
		}
		seen[question.ID] = true
		switch question.Type {
		case model.AppealQuestionShortText, model.AppealQuestionLongText, model.AppealQuestionBoolean:
		default:
			return nil, appealValidation("appeal question type is unsupported")
		}
	}
	return normalized, nil
}

func validateAnswers(questions []model.AppealQuestion, answers []model.AppealAnswer) ([]model.AppealAnswer, error) {
	byID := map[string]model.AppealAnswer{}
	for _, answer := range answers {
		answer.QuestionID = strings.TrimSpace(answer.QuestionID)
		if answer.QuestionID == "" || byID[answer.QuestionID].QuestionID != "" {
			return nil, appealValidation("answers must have unique question ids")
		}
		byID[answer.QuestionID] = answer
	}
	normalized := make([]model.AppealAnswer, 0, len(questions))
	for _, question := range questions {
		answer, present := byID[question.ID]
		if !present {
			if question.Required {
				return nil, appealValidation("required appeal answer is missing")
			}
			continue
		}
		switch question.Type {
		case model.AppealQuestionBoolean:
			if _, ok := answer.Value.(bool); !ok {
				return nil, appealValidation("boolean appeal answer is invalid")
			}
		default:
			value, ok := answer.Value.(string)
			if !ok || len([]rune(strings.TrimSpace(value))) > 4000 || (question.Required && strings.TrimSpace(value) == "") {
				return nil, appealValidation("text appeal answer is invalid")
			}
			answer.Value = strings.TrimSpace(value)
		}
		normalized = append(normalized, answer)
		delete(byID, question.ID)
	}
	if len(byID) != 0 {
		return nil, appealValidation("answer references an unknown question")
	}
	return normalized, nil
}

func decodeQuestions(body string) ([]model.AppealQuestion, error) {
	var questions []model.AppealQuestion
	if err := json.Unmarshal([]byte(body), &questions); err != nil {
		return nil, fmt.Errorf("decode appeal questions: %w", err)
	}
	return validateQuestions(questions)
}

func caseSnapshotAppealable(body string) bool {
	var snapshot struct {
		Template struct {
			Appealable bool `json:"appealable"`
		} `json:"template"`
	}
	return json.Unmarshal([]byte(body), &snapshot) == nil && snapshot.Template.Appealable
}

func validAppealState(status model.AppealStatus) bool {
	switch status {
	case model.AppealStatusPending, model.AppealStatusNeedsInformation, model.AppealStatusAccepted, model.AppealStatusRejected, model.AppealStatusClosed:
		return true
	default:
		return false
	}
}

func requireAppealReview(guildContext *GuildStaffContext) error {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil || !guildContext.Can(model.PermissionActionAppealReview) {
		return ErrAppealPermissionDenied
	}
	return nil
}

func memberNotificationBody(status model.AppealStatus, reason string) string {
	switch status {
	case model.AppealStatusNeedsInformation:
		return "Staff requested more information on your appeal: " + reason
	case model.AppealStatusAccepted:
		return "Your appeal was accepted: " + reason
	case model.AppealStatusRejected:
		return "Your appeal was rejected: " + reason
	default:
		return "Your appeal was closed: " + reason
	}
}

func appealValidation(message string) error {
	return fmt.Errorf("%w: %s", ErrAppealValidation, message)
}

func appealAudit(ctx context.Context, guildID, actorID string, permissionBits uint64, action, resourceType, resourceID string, result model.AuditResult) model.AuditLogEntry {
	requestID, correlationID := TraceIDsFromContext(ctx)
	return model.AuditLogEntry{GuildID: guildID, ActorDiscordUserID: actorID, ActorPermissionBits: permissionBits, Source: model.AuditSourceWeb, Action: action, ResourceType: resourceType, ResourceID: resourceID, Result: result, RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"}
}

func pointerAudit(entry model.AuditLogEntry) *model.AuditLogEntry { return &entry }

func (s *AppealService) auditMember(ctx context.Context, guildID, actorID, action, resourceID string, result model.AuditResult) error {
	entry := appealAudit(ctx, guildID, actorID, 0, action, "appeal", resourceID, result)
	return s.store.CreateAuditLogEntry(ctx, &entry)
}
