package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	actionmods "github.com/quackdiscord/bot/app/actions"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

type ActionService struct {
	store    *storage.Store
	discord  DiscordActionClient
	handlers map[structs.ActionType]actionmods.Executor
}

func NewActionService(store *storage.Store, discord DiscordActionClient) *ActionService {
	return &ActionService{
		store:   store,
		discord: discord,
		handlers: map[structs.ActionType]actionmods.Executor{
			structs.ActionSendDM:      actionmods.SendDM(discord),
			structs.ActionTimeoutUser: actionmods.TimeoutUser(),
			structs.ActionKickUser:    actionmods.KickUser(),
			structs.ActionBanUser:     actionmods.BanUser(),
		},
	}
}

func (s *ActionService) ProcessCaseActions(ctx context.Context, caseID string) error {
	ctx = ensureTraceContext(ctx)
	if s == nil || s.store == nil {
		return errors.New("action service is not configured")
	}

	workerID := actionWorkerID()
	for {
		claimed, err := s.store.ClaimNextCaseAction(ctx, storage.ClaimCaseActionParams{
			CaseID:   strings.TrimSpace(caseID),
			WorkerID: workerID,
		})
		if err != nil {
			return err
		}
		if claimed == nil {
			return nil
		}

		if err := s.processClaimedAction(ctx, workerID, *claimed); err != nil {
			return err
		}
	}
}

func (s *ActionService) processClaimedAction(ctx context.Context, workerID string, claimed storage.ClaimedCaseAction) error {
	handler, ok := s.handlers[claimed.Execution.ActionType]
	if !ok || handler == nil {
		handler = actionmods.Func(actionmods.Unsupported)
	}

	config := parseConfigMap(claimed.Execution.ConfigSnapshotJSON)
	actionContext := actionmods.Context{
		Case:      claimed.Case,
		Execution: claimed.Execution,
		Config:    config,
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

	attemptStatus := structs.ActionAttemptSucceeded
	executionStatus := structs.ActionExecutionSucceeded
	eventType := structs.CaseEventActionSucceeded
	eventBody := fmt.Sprintf("Action %s succeeded", claimed.Execution.ActionType)
	var nextRetryAt *time.Time

	if result.Error != "" {
		attemptStatus = structs.ActionAttemptFailed
		eventType = structs.CaseEventActionFailed
		eventBody = fmt.Sprintf("Action %s failed: %s", claimed.Execution.ActionType, result.Error)
		executionStatus = structs.ActionExecutionFailed
		if shouldRetryAction(claimed.Execution, result) {
			next := nextRetryTime(claimed.Execution)
			nextRetryAt = &next
			executionStatus = structs.ActionExecutionRetrying
			eventBody = fmt.Sprintf("Action %s failed and will retry: %s", claimed.Execution.ActionType, result.Error)
		}
	}

	if result.Response == nil {
		result.Response = map[string]any{}
	}
	if result.Error != "" {
		result.Response["error"] = result.Error
	}

	err := s.store.CompleteCaseAction(ctx, storage.CompleteCaseActionParams{
		ExecutionID:         claimed.Execution.ID,
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
			"retrying":     executionStatus == structs.ActionExecutionRetrying,
		}),
		CorrelationID: correlationID,
		RequestID:     requestID,
	})
	if err != nil {
		return err
	}

	if executionStatus == structs.ActionExecutionFailed && !continueOnError(claimed.Case, claimed.Execution.Position) {
		return s.store.SkipCaseActions(ctx, storage.SkipCaseActionsParams{
			CaseID:        claimed.Case.ID,
			AfterPosition: claimed.Execution.Position,
			Reason:        fmt.Sprintf("previous action %s failed", claimed.Execution.ID),
			CorrelationID: correlationID,
			RequestID:     requestID,
		})
	}
	if executionStatus == structs.ActionExecutionRetrying && nextRetryAt != nil {
		scheduleCaseActions(ctx, claimed.Case.ID, time.Until(*nextRetryAt))
	}

	return nil
}

func (s *ActionService) executeAction(ctx context.Context, handler actionmods.Executor, action actionmods.Context) actionmods.Result {
	result := handler.Execute(ctx, action)
	if result.Error == "" && action.Execution.NotifyUser {
		response, err := s.sendActionNotification(ctx, action)
		if err != nil {
			return actionmods.ResultFromError(err)
		}
		if result.Response == nil {
			result.Response = map[string]any{}
		}
		result.Response["notification"] = response
	}
	return result
}

func (s *ActionService) sendActionNotification(ctx context.Context, action actionmods.Context) (map[string]any, error) {
	if s.discord == nil {
		return nil, DiscordActionError{Code: "discord_unavailable", Message: "discord action client is not configured", Retryable: false}
	}

	message := notificationMessage(action)
	response, err := s.discord.SendDM(ctx, action.Case.TargetDiscordUserID, message)
	if err != nil {
		return nil, err
	}
	if response == nil {
		response = map[string]any{}
	}
	response["type"] = notificationType(action)
	return response, nil
}

func notificationMessage(action actionmods.Context) string {
	message := actionmods.ConfigString(action.Config, "notification_message")
	if message == "" {
		message = actionmods.ConfigString(action.Config, "message")
	}
	if message != "" {
		return message
	}

	switch structs.NotificationType(notificationType(action)) {
	case structs.NotificationWarning:
		return fmt.Sprintf("You received a warning in this server: %s", action.Case.Reason)
	case structs.NotificationTimeout:
		return fmt.Sprintf("You were timed out in this server: %s", action.Case.Reason)
	case structs.NotificationKick:
		return fmt.Sprintf("You were kicked from this server: %s", action.Case.Reason)
	case structs.NotificationBan:
		return fmt.Sprintf("You were banned from this server: %s", action.Case.Reason)
	default:
		return fmt.Sprintf("You received a moderation action in this server: %s", action.Case.Reason)
	}
}

func notificationType(action actionmods.Context) string {
	if action.Execution.NotificationType != "" {
		return action.Execution.NotificationType
	}

	switch action.Execution.ActionType {
	case structs.ActionTimeoutUser:
		return string(structs.NotificationTimeout)
	case structs.ActionKickUser:
		return string(structs.NotificationKick)
	case structs.ActionBanUser:
		return string(structs.NotificationBan)
	default:
		return "moderation"
	}
}

func shouldRetryAction(execution structs.CaseActionExecution, result actionmods.Result) bool {
	if !result.Retryable || !execution.SafeForRetry {
		return false
	}
	return execution.AttemptCount <= execution.MaxRetries
}

func nextRetryTime(execution structs.CaseActionExecution) time.Time {
	backoff := execution.RetryBackoffMS
	if backoff <= 0 {
		backoff = 1000
	}
	return time.Now().UTC().Add(time.Duration(backoff) * time.Millisecond)
}

func continueOnError(caseModel structs.Case, position int) bool {
	var snapshot struct {
		Actions []struct {
			Position        int  `json:"position"`
			ContinueOnError bool `json:"continue_on_error"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(caseModel.TemplateSnapshotJSON), &snapshot); err != nil {
		return false
	}
	for _, action := range snapshot.Actions {
		if action.Position == position {
			return action.ContinueOnError
		}
	}
	return false
}

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

func mustMarshalJSONObject(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func actionWorkerID() string {
	return fmt.Sprintf("action-worker:%d", time.Now().UTC().UnixNano())
}
