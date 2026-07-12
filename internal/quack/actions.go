package quack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	actionmods "github.com/quackdiscord/bot/internal/quack/actionmods"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// ActionService claims persisted actions, executes supported moderation behavior, and records retry or terminal outcomes.
type ActionService struct {
	store            Repository
	discord          DiscordActionClient
	handlers         map[model.ActionType]actionmods.Executor
	authorizer       *GuildService
	scheduler        CaseWorkScheduler
	dashboardBaseURL string
}

// WithDashboardBaseURL configures the secure member entry point used by
// appealable case notifications.
func (s *ActionService) WithDashboardBaseURL(baseURL string) *ActionService {
	if s != nil {
		s.dashboardBaseURL = strings.TrimSpace(baseURL)
	}
	return s
}

// NewActionService constructs action service with required dependencies explicit so callers control lifecycle and substitution.
func NewActionService(store Repository, discord DiscordActionClient) *ActionService {
	return &ActionService{
		store:   store,
		discord: discord,
		handlers: map[model.ActionType]actionmods.Executor{
			model.ActionSendDM:        actionmods.SendDM(discord),
			model.ActionTimeoutUser:   actionmods.TimeoutUser(discord),
			model.ActionKickUser:      actionmods.KickUser(discord),
			model.ActionBanUser:       actionmods.BanUser(discord),
			model.ActionRemoveTimeout: actionmods.RemoveTimeout(discord),
			model.ActionUnbanUser:     actionmods.UnbanUser(discord),
		},
	}
}

// WithRecoveryControls configures live authorization and scheduling for manual retries and reversals.
func (s *ActionService) WithRecoveryControls(authorizer *GuildService, scheduler CaseWorkScheduler) *ActionService {
	if s != nil {
		s.authorizer = authorizer
		s.scheduler = scheduler
	}
	return s
}

// ProcessCaseActions processes case actions according to persisted state and retry policy.
func (s *ActionService) ProcessCaseActions(ctx context.Context, caseID string) error {
	ctx = ensureTraceContext(ctx)
	if s == nil || s.store == nil {
		return errors.New("action service is not configured")
	}

	workerID := actionWorkerID()
	for {
		claimed, err := s.store.ClaimNextCaseAction(ctx, model.ClaimCaseActionParams{
			CaseID:   strings.TrimSpace(caseID),
			WorkerID: workerID,
		})
		if err != nil {
			return err
		}
		if claimed == nil {
			return s.processNotification(ctx, workerID, strings.TrimSpace(caseID))
		}
		if claimed.Execution.ActionType == model.ActionKickUser || claimed.Execution.ActionType == model.ActionBanUser {
			s.prepareNotification(ctx, claimed.Case)
		}

		if err := s.processClaimedAction(ctx, workerID, *claimed); err != nil {
			return err
		}
	}
}

// processClaimedAction processes claimed action according to persisted state and retry policy.
func (s *ActionService) processClaimedAction(ctx context.Context, workerID string, claimed model.ClaimedCaseAction) error {
	handler, ok := s.handlers[claimed.Execution.ActionType]
	if !ok || handler == nil {
		handler = actionmods.Func(actionmods.Unsupported)
	}

	config := parseConfigMap(claimed.Execution.ConfigSnapshotJSON)
	guild, _ := s.store.GetGuildByID(ctx, claimed.Case.GuildID)
	discordGuildID := ""
	if guild != nil {
		discordGuildID = guild.DiscordGuildID
	}
	actionContext := actionmods.Context{
		Case:           claimed.Case,
		Execution:      claimed.Execution,
		Config:         config,
		DiscordGuildID: discordGuildID,
	}
	result := s.executeAction(ctx, handler, actionContext)
	requestPayload := map[string]any{
		"case_id":      claimed.Case.ID,
		"execution_id": claimed.Execution.ID,
		"action_type":  claimed.Execution.ActionType,
		"config":       config,
	}
	requestID, correlationID := TraceIDsFromContext(ctx)
	if correlationID == "" {
		correlationID = claimed.Case.CorrelationID
	}

	attemptStatus := model.ActionAttemptSucceeded
	executionStatus := model.ActionExecutionSucceeded
	eventType := model.CaseEventActionSucceeded
	eventBody := "Discord enforcement succeeded"
	var nextRetryAt *time.Time

	if result.Error != "" {
		attemptStatus = model.ActionAttemptFailed
		eventType = model.CaseEventActionFailed
		eventBody = "Discord enforcement failed and requires staff review"
		executionStatus = model.ActionExecutionFailed
		if shouldRetryAction(claimed.Execution, result) {
			next := nextRetryTime(claimed.Execution)
			nextRetryAt = &next
			executionStatus = model.ActionExecutionRetrying
			eventBody = "Discord enforcement is waiting for a safe automatic retry"
		}
	}

	if result.Response == nil {
		result.Response = map[string]any{}
	}
	if result.Error != "" {
		result.Response["error"] = result.Error
	}

	err := s.store.CompleteCaseAction(ctx, model.CompleteCaseActionParams{
		ExecutionID:         claimed.Execution.ID,
		LeaseToken:          claimed.Execution.LeaseToken,
		AttemptNumber:       claimed.Execution.AttemptCount,
		WorkerID:            workerID,
		AttemptStatus:       attemptStatus,
		ExecutionStatus:     executionStatus,
		ErrorCode:           result.ErrorCode,
		ErrorMessage:        result.Error,
		RequestPayloadJSON:  mustMarshalJSONObject(requestPayload),
		ResponsePayloadJSON: mustMarshalJSONObject(result.Response),
		NextRetryAt:         nextRetryAt,
		EventType:           eventType,
		EventBody:           eventBody,
		EventMetadataJSON: mustMarshalJSONObject(map[string]any{
			"execution_id": claimed.Execution.ID,
			"action_type":  claimed.Execution.ActionType,
			"retrying":     executionStatus == model.ActionExecutionRetrying,
		}),
		CorrelationID: correlationID,
		RequestID:     requestID,
	})
	if err != nil {
		return err
	}

	return nil
}

// executeAction processes action according to persisted state and retry policy.
func (s *ActionService) executeAction(ctx context.Context, handler actionmods.Executor, action actionmods.Context) actionmods.Result {
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return handler.Execute(requestCtx, action)
}

// shouldRetryAction encapsulates the should retry action rule so callers share one consistent package implementation.
func shouldRetryAction(execution model.CaseActionExecution, result actionmods.Result) bool {
	if !result.Retryable || result.OutcomeUncertain || !execution.SafeForRetry {
		return false
	}
	return execution.AttemptCount <= execution.MaxRetries
}

// prepareNotification opens a DM channel before an irreversible membership change without coupling preparation failure to enforcement.
func (s *ActionService) prepareNotification(ctx context.Context, item model.Case) {
	notification, err := s.store.GetCaseNotification(ctx, item.ID)
	if err != nil || notification == nil || notification.Status != model.NotificationPending {
		return
	}
	prepared, ok := s.discord.(DiscordPreparedDMClient)
	if !ok {
		_ = s.store.PrepareCaseNotification(ctx, item.ID, "", "prepared DM adapter is unavailable")
		return
	}
	channelID, prepareErr := prepared.PrepareDM(ctx, item.TargetDiscordUserID)
	message := ""
	if prepareErr != nil {
		message = redactDiscordError(prepareErr)
	}
	_ = s.store.PrepareCaseNotification(ctx, item.ID, channelID, message)
}

// processNotification renders and attempts the one case-level notification after enforcement reaches a terminal outcome.
func (s *ActionService) processNotification(ctx context.Context, workerID, caseID string) error {
	claimed, err := s.store.ClaimCaseNotification(ctx, model.ClaimCaseNotificationParams{CaseID: caseID, WorkerID: workerID})
	if err != nil || claimed == nil {
		return err
	}
	item, err := s.store.GetCaseByID(ctx, caseID)
	if err != nil || item == nil {
		return err
	}
	guild, err := s.store.GetGuildByID(ctx, item.GuildID)
	if err != nil {
		return err
	}
	settings, err := s.store.GetGuildSettings(ctx, item.GuildID)
	if err != nil {
		return err
	}
	actions, err := s.store.ListCaseActionExecutions(ctx, item.ID)
	if err != nil {
		return err
	}
	message := renderCaseNotification(*item, guild, settings, actions)
	if err := s.store.BeginCaseNotificationDelivery(ctx, claimed.ID, claimed.LeaseToken); err != nil {
		return err
	}
	var response map[string]any
	var sendErr error
	appealable := caseSnapshotAppealable(item.TemplateSnapshotJSON)
	if appealable && s.dashboardBaseURL != "" {
		if client, ok := s.discord.(DiscordCaseNotificationClient); ok {
			response, sendErr = client.SendCaseNotification(ctx, item.TargetDiscordUserID, claimed.PreparedChannelDiscordID, message, s.dashboardBaseURL, item.GuildID, item.ID)
		} else {
			sendErr = errors.New("Discord appeal notification adapter is unavailable")
		}
	} else if claimed.PreparedChannelDiscordID != "" {
		if prepared, ok := s.discord.(DiscordPreparedDMClient); ok {
			response, sendErr = prepared.SendPreparedDM(ctx, claimed.PreparedChannelDiscordID, message)
		} else {
			sendErr = errors.New("prepared DM adapter is unavailable")
		}
	} else if s.discord != nil {
		response, sendErr = s.discord.SendDM(ctx, item.TargetDiscordUserID, message)
	} else {
		sendErr = errors.New("Discord action client is unavailable")
	}
	params := model.CompleteCaseNotificationParams{NotificationID: claimed.ID, LeaseToken: claimed.LeaseToken, WorkerID: workerID, RenderedMessage: message, PreparedChannelDiscordID: claimed.PreparedChannelDiscordID}
	if sendErr != nil {
		result := actionmods.ResultFromError(sendErr)
		params.Status = model.NotificationFailed
		params.ErrorCode = result.ErrorCode
		params.ErrorMessage = result.Error
		params.EventType = model.CaseEventNotificationFailed
	} else {
		params.Status = model.NotificationSent
		params.EventType = model.CaseEventNotificationSent
		if response != nil {
			params.DeliveryMessageDiscordID = fmt.Sprint(response["message_id"])
		}
	}
	return s.store.CompleteCaseNotification(ctx, params)
}

// renderCaseNotification builds the bounded product-owned message without executable guild templates.
func renderCaseNotification(item model.Case, guild *model.Guild, settings *model.GuildSettings, actions []model.CaseActionExecution) string {
	guildName := "this server"
	if guild != nil && strings.TrimSpace(guild.Name) != "" {
		guildName = guild.Name
	}
	parts := []string{}
	if settings != nil && strings.TrimSpace(settings.NotificationIntroduction) != "" {
		parts = append(parts, truncateRunes(strings.TrimSpace(settings.NotificationIntroduction), 150))
	}
	parts = append(parts, fmt.Sprintf("Moderation case #%d in %s", item.CaseNumber, guildName), "Reason: "+truncateRunes(item.Reason, 200))
	for _, value := range parseCaseContextValues(item.ContextValuesJSON) {
		if value.Value != nil {
			parts = append(parts, fmt.Sprintf("%s: %s", truncateRunes(value.Label, 40), truncateRunes(fmt.Sprint(value.Value), 50)))
		}
	}
	outcome := "No Discord enforcement action was configured."
	if len(actions) > 0 {
		action := actions[0]
		outcome = fmt.Sprintf("Outcome: %s (%s)", action.ActionType, action.Status)
	}
	parts = append(parts, outcome, fmt.Sprintf("Case reference: %s/%d", item.GuildID, item.CaseNumber))
	if snapshot := templateSnapshotResponse(item.TemplateSnapshotJSON); snapshot != nil && snapshot.Template.Appealable {
		parts = append(parts, "This case can be appealed from your Quack dashboard.")
	}
	if settings != nil && strings.TrimSpace(settings.NotificationFooter) != "" {
		parts = append(parts, truncateRunes(strings.TrimSpace(settings.NotificationFooter), 150))
	}
	return truncateRunes(strings.Join(parts, "\n"), 2000)
}

// redactDiscordError converts adapter failures to safe durable notification diagnostics.
func redactDiscordError(err error) string {
	if err == nil {
		return ""
	}
	var discordErr actionmods.DiscordError
	if errors.As(err, &discordErr) {
		return discordErr.Error()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Discord request timed out"
	}
	return "Discord request failed"
}

// ListFailures returns the active failed-action review queue.
func (s *ActionService) ListFailures(ctx context.Context, guildContext *GuildStaffContext, limit, offset int) (*model.FailedCaseActionResult, error) {
	if guildContext == nil || guildContext.Guild == nil || !guildContext.Can(model.PermissionActionCaseRead) {
		if s != nil && s.store != nil && guildContext != nil && guildContext.Guild != nil && guildContext.Staff != nil {
			entry := actionControlAudit(ctx, guildContext, string(model.AuditActionActionFailureRead), "list")
			entry.Result = model.AuditResultDenied
			entry.FailureReason = "permission_denied"
			_ = s.store.CreateAuditLogEntry(ctx, entry)
		}
		return nil, ErrCasePermissionDenied
	}
	result, err := s.store.ListFailedCaseActions(ctx, model.FailedCaseActionFilter{GuildID: guildContext.Guild.ID, Limit: limit, Offset: offset})
	entry := actionControlAudit(ctx, guildContext, string(model.AuditActionActionFailureRead), "list")
	if err != nil {
		entry.Result = model.AuditResultFailure
		entry.FailureReason = "query_failed"
	} else {
		entry.Result = model.AuditResultSuccess
	}
	if auditErr := s.store.CreateAuditLogEntry(ctx, entry); auditErr != nil && err == nil {
		return nil, auditErr
	}
	return result, err
}

// Retry performs live preflight before requeueing the immutable failed action.
func (s *ActionService) Retry(ctx context.Context, guildContext *GuildStaffContext, executionID string) (updated *model.CaseActionExecution, err error) {
	defer func() {
		s.auditControlFailure(ctx, guildContext, string(model.AuditActionActionRetry), executionID, err)
	}()
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, ErrCasePermissionDenied
	}
	execution, err := s.store.GetCaseActionExecution(ctx, guildContext.Guild.ID, executionID)
	if err != nil {
		return nil, err
	}
	if execution == nil || execution.DismissedAt != nil {
		return nil, ErrCaseNotFound
	}
	if execution.Status == model.ActionExecutionPending || execution.Status == model.ActionExecutionRetrying {
		return execution, nil
	}
	if execution.Status != model.ActionExecutionFailed {
		return nil, ErrCaseNotFound
	}
	item, err := s.store.GetCaseByID(ctx, execution.CaseID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrCaseNotFound
	}
	if s.authorizer == nil {
		return nil, ErrAuthorizationUnavailable
	}
	if err := s.authorizer.PreflightCase(ctx, guildContext, item.TargetDiscordUserID, execution.ActionType); err != nil {
		return nil, err
	}
	updated, err = s.store.RetryCaseAction(ctx, model.RetryCaseActionParams{GuildID: item.GuildID, ExecutionID: execution.ID, ActorDiscordUserID: guildContext.Staff.DiscordUserID, Audit: actionControlAudit(ctx, guildContext, "case_action.retry", execution.ID)})
	if err == nil && updated != nil && s.scheduler != nil {
		s.scheduler.Submit(ctx, item.ID)
	}
	return updated, err
}

// Dismiss preserves attempts while removing a failure from active staff review.
func (s *ActionService) Dismiss(ctx context.Context, guildContext *GuildStaffContext, executionID string) (updated *model.CaseActionExecution, err error) {
	defer func() {
		s.auditControlFailure(ctx, guildContext, string(model.AuditActionActionDismiss), executionID, err)
	}()
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil || !guildContext.Can(model.PermissionActionFailureDismiss) {
		return nil, ErrCasePermissionDenied
	}
	return s.store.DismissCaseAction(ctx, model.DismissCaseActionParams{GuildID: guildContext.Guild.ID, ExecutionID: executionID, ActorDiscordUserID: guildContext.Staff.DiscordUserID, Audit: actionControlAudit(ctx, guildContext, "case_action.dismiss", executionID)})
}

// Reverse queues a matching staff-confirmed timeout removal or unban after live permission checks.
func (s *ActionService) Reverse(ctx context.Context, guildContext *GuildStaffContext, caseID, originalExecutionID string, actionType model.ActionType) (*model.CaseActionExecution, error) {
	return s.ReverseForAppeal(ctx, guildContext, caseID, originalExecutionID, actionType, nil)
}

// ReverseForAppeal queues a reversal and, when supplied, verifies its accepted case-linked appeal.
func (s *ActionService) ReverseForAppeal(ctx context.Context, guildContext *GuildStaffContext, caseID, originalExecutionID string, actionType model.ActionType, appealID *string) (queued *model.CaseActionExecution, err error) {
	defer func() {
		s.auditControlFailure(ctx, guildContext, string(model.AuditActionActionReverse), originalExecutionID, err)
	}()
	if s.authorizer == nil {
		return nil, ErrAuthorizationUnavailable
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil, ErrCaseNotFound
	}
	item, err := s.store.GetCaseByIDOrNumber(ctx, guildContext.Guild.ID, caseID)
	if err != nil {
		return nil, err
	}
	if item == nil || item.GuildID != guildContext.Guild.ID {
		return nil, ErrCaseNotFound
	}
	if err := s.authorizer.PreflightReversal(ctx, guildContext, item.TargetDiscordUserID, actionType); err != nil {
		return nil, err
	}
	queued, err = s.store.QueueCaseReversal(ctx, model.QueueCaseReversalParams{GuildID: item.GuildID, CaseID: item.ID, ActorDiscordUserID: guildContext.Staff.DiscordUserID, OriginalExecutionID: originalExecutionID, ActionType: actionType, AppealID: appealID, Audit: actionControlAudit(ctx, guildContext, "case_action.reverse", originalExecutionID)})
	if err == nil && queued != nil && s.scheduler != nil {
		s.scheduler.Submit(ctx, item.ID)
	}
	return queued, err
}

func (s *ActionService) auditControlFailure(ctx context.Context, guildContext *GuildStaffContext, action, resourceID string, operationErr error) {
	if operationErr == nil || s == nil || s.store == nil || guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return
	}
	entry := actionControlAudit(ctx, guildContext, action, resourceID)
	entry.Result = model.AuditResultFailure
	if errors.Is(operationErr, ErrCasePermissionDenied) || errors.Is(operationErr, ErrAuthorizationDenied) {
		entry.Result = model.AuditResultDenied
	}
	entry.FailureReason = operationErr.Error()
	_ = s.store.CreateAuditLogEntry(ctx, entry)
}

// actionControlAudit constructs immutable staff recovery evidence with current permission bits.
func actionControlAudit(ctx context.Context, guildContext *GuildStaffContext, action, resourceID string) *model.AuditLogEntry {
	requestID, correlationID := TraceIDsFromContext(ctx)
	return &model.AuditLogEntry{GuildID: guildContext.Guild.ID, ActorDiscordUserID: guildContext.Staff.DiscordUserID, ActorPermissionBits: guildContext.PermissionBits, Source: AuditSourceFromContext(ctx), Action: action, ResourceType: "case_action_execution", ResourceID: resourceID, Result: model.AuditResultSuccess, RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"}
}

// nextRetryTime encapsulates the next retry time rule so callers share one consistent package implementation.
func nextRetryTime(execution model.CaseActionExecution) time.Time {
	backoff := execution.RetryBackoffMS
	if backoff <= 0 {
		backoff = 1000
	}
	return time.Now().UTC().Add(time.Duration(backoff) * time.Millisecond)
}

// parseConfigMap parses config map and rejects malformed input before it reaches core logic.
func parseConfigMap(body string) map[string]any {
	if strings.TrimSpace(body) == "" {
		return map[string]any{}
	}

	var config map[string]any
	if err := json.Unmarshal([]byte(body), &config); err != nil || config == nil {
		return map[string]any{}
	}
	return config
}

// mustMarshalJSONObject serializes must marshal jsonobject into its stable external representation.
func mustMarshalJSONObject(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}

// actionWorkerID encapsulates the action worker id rule so callers share one consistent package implementation.
func actionWorkerID() string {
	return fmt.Sprintf("action-worker:%d", time.Now().UTC().UnixNano())
}
