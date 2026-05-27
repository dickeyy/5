package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/quackdiscord/bot/structs"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CreateCaseParams struct {
	Case             structs.Case
	Event            structs.CaseEvent
	ActionExecutions []structs.CaseActionExecution
	Audit            *structs.AuditLogEntry
}

type CreatedCase struct {
	Case             structs.Case
	Event            structs.CaseEvent
	ActionExecutions []structs.CaseActionExecution
}

type CountTemplateCasesForTargetParams struct {
	GuildID             string
	TemplateID          string
	TargetDiscordUserID string
	Since               *time.Time
}

type ListCasesParams struct {
	GuildID                string
	TargetDiscordUserID    string
	ModeratorDiscordUserID string
	TemplateID             string
	Status                 structs.CaseStatus
	Limit                  int
	Offset                 int
}

type ListCasesResult struct {
	Cases []structs.Case
	Total int64
}

type TargetCaseSummary struct {
	Total      int64
	ByStatus   map[structs.CaseStatus]int64
	ByTemplate map[string]int64
}

type ClaimedCaseAction struct {
	Case      structs.Case
	Execution structs.CaseActionExecution
}

type ClaimCaseActionParams struct {
	CaseID   string
	WorkerID string
}

type CompleteCaseActionParams struct {
	ExecutionID         string
	AttemptNumber       uint8
	WorkerID            string
	AttemptStatus       structs.ActionAttemptStatus
	ExecutionStatus     structs.ActionExecutionStatus
	ErrorCode           string
	ErrorMessage        string
	RequestPayloadJSON  string
	ResponsePayloadJSON string
	NextRetryAt         *time.Time
	EventType           structs.CaseEventType
	EventBody           string
	EventMetadataJSON   string
	CorrelationID       string
	RequestID           string
}

type SkipCaseActionsParams struct {
	CaseID        string
	AfterPosition int
	Reason        string
	CorrelationID string
	RequestID     string
}

func (s *Store) CreateCase(ctx context.Context, params CreateCaseParams) (*CreatedCase, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	caseModel := params.Case
	if err := prepareULIDModel(&caseModel.ULIDModel, now); err != nil {
		return nil, fmt.Errorf("prepare case model: %w", err)
	}
	if caseModel.Status == "" {
		caseModel.Status = structs.CaseStatusOpen
	}
	if caseModel.Source == "" {
		caseModel.Source = structs.CaseSourceAPI
	}
	if caseModel.TemplateSnapshotJSON == "" {
		caseModel.TemplateSnapshotJSON = "{}"
	}
	if caseModel.MetadataJSON == "" {
		caseModel.MetadataJSON = "{}"
	}

	event := params.Event
	actionExecutions := make([]structs.CaseActionExecution, len(params.ActionExecutions))
	copy(actionExecutions, params.ActionExecutions)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		caseNumber, err := nextCaseNumber(tx, caseModel.GuildID)
		if err != nil {
			return err
		}
		caseModel.CaseNumber = caseNumber

		if err := tx.Select("*").Create(&caseModel).Error; err != nil {
			return fmt.Errorf("create case: %w", err)
		}

		event.CaseID = caseModel.ID
		event.GuildID = caseModel.GuildID
		if event.EventType == "" {
			event.EventType = structs.CaseEventCreated
		}
		if event.ActorType == "" {
			event.ActorType = "staff"
		}
		if event.Visibility == "" {
			event.Visibility = structs.EventVisibilityStaff
		}
		if event.MetadataJSON == "" {
			event.MetadataJSON = "{}"
		}
		if err := prepareULIDModel(&event.ULIDModel, now); err != nil {
			return fmt.Errorf("prepare case event model: %w", err)
		}
		if err := tx.Select("*").Create(&event).Error; err != nil {
			return fmt.Errorf("create case event: %w", err)
		}

		for i := range actionExecutions {
			actionExecutions[i].CaseID = caseModel.ID
			if actionExecutions[i].Status == "" {
				actionExecutions[i].Status = structs.ActionExecutionPending
			}
			if actionExecutions[i].ConfigSnapshotJSON == "" {
				actionExecutions[i].ConfigSnapshotJSON = "{}"
			}
			if actionExecutions[i].IdempotencyKey == "" {
				actionExecutions[i].IdempotencyKey = fmt.Sprintf("case:%s:action:%d", caseModel.ID, actionExecutions[i].Position)
			}
			if actionExecutions[i].CorrelationID == "" {
				actionExecutions[i].CorrelationID = caseModel.CorrelationID
			}
			if err := prepareULIDModel(&actionExecutions[i].ULIDModel, now); err != nil {
				return fmt.Errorf("prepare case action execution model: %w", err)
			}
		}
		if len(actionExecutions) > 0 {
			if err := tx.Select("*").Create(&actionExecutions).Error; err != nil {
				return fmt.Errorf("create case action executions: %w", err)
			}
		}

		if params.Audit != nil {
			audit := *params.Audit
			audit.ResourceID = caseModel.ID
			if err := createAuditLogEntry(tx, &audit, now); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &CreatedCase{
		Case:             caseModel,
		Event:            event,
		ActionExecutions: actionExecutions,
	}, nil
}

func (s *Store) CountTemplateCasesForTarget(ctx context.Context, params CountTemplateCasesForTargetParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("database not connected")
	}

	query := s.db.WithContext(ctx).
		Model(&structs.Case{}).
		Where("guild_id = ?", params.GuildID).
		Where("template_id = ?", params.TemplateID).
		Where("target_discord_user_id = ?", params.TargetDiscordUserID).
		Where("status <> ?", structs.CaseStatusVoided)
	if params.Since != nil {
		query = query.Where("created_at >= ?", *params.Since)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count template cases for target: %w", err)
	}

	return count, nil
}

func (s *Store) ListCases(ctx context.Context, guildID string) ([]structs.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var cases []structs.Case
	if err := s.db.WithContext(ctx).Where("guild_id = ?", guildID).Order("case_number ASC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}

	return cases, nil
}

func (s *Store) ListCasesFiltered(ctx context.Context, params ListCasesParams) (*ListCasesResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	var total int64
	if err := filteredCasesQuery(s.db.WithContext(ctx).Model(&structs.Case{}), params).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count cases: %w", err)
	}

	var cases []structs.Case
	if err := filteredCasesQuery(s.db.WithContext(ctx).Model(&structs.Case{}), params).
		Order("case_number DESC").
		Limit(limit).
		Offset(offset).
		Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list filtered cases: %w", err)
	}

	return &ListCasesResult{Cases: cases, Total: total}, nil
}

func (s *Store) GetCaseByIDOrNumber(ctx context.Context, guildID, caseRef string) (*structs.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	query := s.db.WithContext(ctx).Where("guild_id = ?", guildID)
	if caseNumber, err := strconv.ParseUint(caseRef, 10, 64); err == nil && strconv.FormatUint(caseNumber, 10) == caseRef {
		query = query.Where("case_number = ?", caseNumber)
	} else {
		query = query.Where("id = ?", caseRef)
	}

	var caseModel structs.Case
	if err := query.First(&caseModel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get case by id or number: %w", err)
	}

	return &caseModel, nil
}

func (s *Store) TargetCaseSummary(ctx context.Context, guildID, targetDiscordUserID string) (*TargetCaseSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	summary := &TargetCaseSummary{
		ByStatus:   map[structs.CaseStatus]int64{},
		ByTemplate: map[string]int64{},
	}

	targetCases := func() *gorm.DB {
		return s.db.WithContext(ctx).Model(&structs.Case{}).
			Where("guild_id = ?", guildID).
			Where("target_discord_user_id = ?", targetDiscordUserID)
	}
	if err := targetCases().Count(&summary.Total).Error; err != nil {
		return nil, fmt.Errorf("count target cases: %w", err)
	}

	var statusRows []struct {
		Status structs.CaseStatus
		Count  int64
	}
	if err := targetCases().Select("status, COUNT(*) AS count").Group("status").Scan(&statusRows).Error; err != nil {
		return nil, fmt.Errorf("count target cases by status: %w", err)
	}
	for _, row := range statusRows {
		summary.ByStatus[row.Status] = row.Count
	}

	var templateRows []struct {
		TemplateID string
		Count      int64
	}
	if err := targetCases().Select("COALESCE(template_id, '') AS template_id, COUNT(*) AS count").Group("template_id").Scan(&templateRows).Error; err != nil {
		return nil, fmt.Errorf("count target cases by template: %w", err)
	}
	for _, row := range templateRows {
		summary.ByTemplate[row.TemplateID] = row.Count
	}

	return summary, nil
}

func (s *Store) ListCaseEvents(ctx context.Context, caseID string) ([]structs.CaseEvent, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var events []structs.CaseEvent
	if err := s.db.WithContext(ctx).Where("case_id = ?", caseID).Order("created_at ASC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list case events: %w", err)
	}

	return events, nil
}

func (s *Store) ListCaseActionExecutions(ctx context.Context, caseID string) ([]structs.CaseActionExecution, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var executions []structs.CaseActionExecution
	if err := s.db.WithContext(ctx).Where("case_id = ?", caseID).Order("position ASC").Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("list case action executions: %w", err)
	}

	return executions, nil
}

func (s *Store) ListCaseActionAttempts(ctx context.Context, executionIDs []string) ([]structs.CaseActionAttempt, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	if len(executionIDs) == 0 {
		return nil, nil
	}

	var attempts []structs.CaseActionAttempt
	if err := s.db.WithContext(ctx).
		Where("execution_id IN ?", executionIDs).
		Order("execution_id ASC, attempt_number ASC").
		Find(&attempts).Error; err != nil {
		return nil, fmt.Errorf("list case action attempts: %w", err)
	}

	return attempts, nil
}

func filteredCasesQuery(query *gorm.DB, params ListCasesParams) *gorm.DB {
	query = query.Where("guild_id = ?", params.GuildID)
	if params.TargetDiscordUserID != "" {
		query = query.Where("target_discord_user_id = ?", params.TargetDiscordUserID)
	}
	if params.ModeratorDiscordUserID != "" {
		query = query.Where("moderator_discord_user_id = ?", params.ModeratorDiscordUserID)
	}
	if params.TemplateID != "" {
		query = query.Where("template_id = ?", params.TemplateID)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	return query
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nextCaseNumber(tx *gorm.DB, guildID string) (uint64, error) {
	var latest structs.Case
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("guild_id = ?", guildID).
		Order("case_number DESC").
		Limit(1).
		Find(&latest).Error
	if err != nil {
		return 0, fmt.Errorf("get latest case number: %w", err)
	}
	if latest.CaseNumber == 0 {
		return 1, nil
	}
	return latest.CaseNumber + 1, nil
}

func (s *Store) ClaimNextCaseAction(ctx context.Context, params ClaimCaseActionParams) (*ClaimedCaseAction, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	var claimed *ClaimedCaseAction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var caseModel structs.Case
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", params.CaseID).
			Limit(1).
			Find(&caseModel)
		if result.Error != nil {
			return fmt.Errorf("get case for action claim: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}

		var running int64
		if err := tx.Model(&structs.CaseActionExecution{}).
			Where("case_id = ? AND status = ?", params.CaseID, structs.ActionExecutionRunning).
			Count(&running).Error; err != nil {
			return fmt.Errorf("count running case actions: %w", err)
		}
		if running > 0 {
			return nil
		}

		var execution structs.CaseActionExecution
		result = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("case_id = ? AND status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)",
				params.CaseID,
				[]structs.ActionExecutionStatus{structs.ActionExecutionPending, structs.ActionExecutionRetrying},
				now,
			).
			Order("position ASC").
			Limit(1).
			Find(&execution)
		if result.Error != nil {
			return fmt.Errorf("claim case action execution: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}

		execution.Status = structs.ActionExecutionRunning
		execution.AttemptCount++
		execution.StartedAt = &now
		execution.FinishedAt = nil
		execution.NextRetryAt = nil
		execution.UpdatedAt = now
		if err := tx.Select("*").Save(&execution).Error; err != nil {
			return fmt.Errorf("mark case action running: %w", err)
		}

		if caseModel.Status == structs.CaseStatusOpen {
			caseModel.Status = structs.CaseStatusActionRunning
			caseModel.UpdatedAt = now
			if err := tx.Save(&caseModel).Error; err != nil {
				return fmt.Errorf("mark case action running: %w", err)
			}
		}

		claimed = &ClaimedCaseAction{
			Case:      caseModel,
			Execution: execution,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return claimed, nil
}

func (s *Store) CompleteCaseAction(ctx context.Context, params CompleteCaseActionParams) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var execution structs.CaseActionExecution
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", params.ExecutionID).
			Limit(1).
			Find(&execution)
		if result.Error != nil {
			return fmt.Errorf("get case action execution: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return nil
		}

		if params.RequestPayloadJSON == "" {
			params.RequestPayloadJSON = "{}"
		}
		if params.ResponsePayloadJSON == "" {
			params.ResponsePayloadJSON = "{}"
		}

		startedAt := now
		if execution.StartedAt != nil {
			startedAt = *execution.StartedAt
		}
		attempt := structs.CaseActionAttempt{
			ExecutionID:         execution.ID,
			AttemptNumber:       params.AttemptNumber,
			Status:              params.AttemptStatus,
			WorkerID:            params.WorkerID,
			StartedAt:           startedAt,
			FinishedAt:          &now,
			DurationMS:          now.Sub(startedAt).Milliseconds(),
			ErrorCode:           params.ErrorCode,
			ErrorMessage:        params.ErrorMessage,
			RequestPayloadJSON:  params.RequestPayloadJSON,
			ResponsePayloadJSON: params.ResponsePayloadJSON,
		}
		if attempt.AttemptNumber == 0 {
			attempt.AttemptNumber = execution.AttemptCount
		}
		if err := prepareULIDModel(&attempt.ULIDModel, now); err != nil {
			return fmt.Errorf("prepare case action attempt model: %w", err)
		}
		if err := tx.Select("*").Create(&attempt).Error; err != nil {
			return fmt.Errorf("create case action attempt: %w", err)
		}

		execution.Status = params.ExecutionStatus
		execution.LastErrorCode = params.ErrorCode
		execution.LastError = params.ErrorMessage
		execution.FinishedAt = &now
		execution.NextRetryAt = params.NextRetryAt
		execution.UpdatedAt = now
		if params.ExecutionStatus == structs.ActionExecutionRetrying {
			execution.FinishedAt = nil
		}
		if err := tx.Select("*").Save(&execution).Error; err != nil {
			return fmt.Errorf("update case action execution: %w", err)
		}

		if params.EventType != "" {
			event := structs.CaseEvent{
				CaseID:       execution.CaseID,
				EventType:    params.EventType,
				ActorType:    "system",
				Visibility:   structs.EventVisibilityStaff,
				Body:         params.EventBody,
				MetadataJSON: params.EventMetadataJSON,
			}
			if event.MetadataJSON == "" {
				event.MetadataJSON = "{}"
			}
			if err := appendCaseEvent(tx, &event, now); err != nil {
				return err
			}
		}

		if err := createCaseActionAudit(tx, execution, params, now); err != nil {
			return err
		}

		return updateCaseStatusFromActions(tx, execution.CaseID, now)
	})
}

func (s *Store) SkipCaseActions(ctx context.Context, params SkipCaseActionsParams) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var executions []structs.CaseActionExecution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("case_id = ? AND position > ? AND status IN ?", params.CaseID, params.AfterPosition, []structs.ActionExecutionStatus{structs.ActionExecutionPending, structs.ActionExecutionRetrying}).
			Order("position ASC").
			Find(&executions).Error; err != nil {
			return fmt.Errorf("list case actions to skip: %w", err)
		}

		for i := range executions {
			executions[i].Status = structs.ActionExecutionSkipped
			executions[i].LastErrorCode = "blocked_by_previous_action"
			executions[i].LastError = params.Reason
			executions[i].FinishedAt = &now
			executions[i].NextRetryAt = nil
			executions[i].UpdatedAt = now
			if err := tx.Select("*").Save(&executions[i]).Error; err != nil {
				return fmt.Errorf("skip case action execution: %w", err)
			}
			if err := createSkippedCaseActionAudit(tx, executions[i], params, now); err != nil {
				return err
			}
		}

		return updateCaseStatusFromActions(tx, params.CaseID, now)
	})
}

func createCaseActionAudit(tx *gorm.DB, execution structs.CaseActionExecution, params CompleteCaseActionParams, now time.Time) error {
	var caseModel structs.Case
	result := tx.Where("id = ?", execution.CaseID).Limit(1).Find(&caseModel)
	if result.Error != nil {
		return fmt.Errorf("get case for action audit: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	action := "case_action.succeeded"
	resultValue := structs.AuditResultSuccess
	switch params.ExecutionStatus {
	case structs.ActionExecutionRetrying:
		action = "case_action.retrying"
		resultValue = structs.AuditResultFailure
	case structs.ActionExecutionFailed:
		action = "case_action.failed"
		resultValue = structs.AuditResultFailure
	}

	return createAuditLogEntry(tx, &structs.AuditLogEntry{
		GuildID:       caseModel.GuildID,
		Source:        structs.AuditSourceSystem,
		Action:        action,
		ResourceType:  "case_action_execution",
		ResourceID:    execution.ID,
		Result:        resultValue,
		FailureReason: params.ErrorMessage,
		CorrelationID: firstNonEmpty(params.CorrelationID, execution.CorrelationID, caseModel.CorrelationID),
		RequestID:     params.RequestID,
		MetadataJSON: marshalJSONObject(map[string]any{
			"case_id":        caseModel.ID,
			"case_number":    caseModel.CaseNumber,
			"action_type":    execution.ActionType,
			"attempt_number": params.AttemptNumber,
			"retrying":       params.ExecutionStatus == structs.ActionExecutionRetrying,
		}),
	}, now)
}

func createSkippedCaseActionAudit(tx *gorm.DB, execution structs.CaseActionExecution, params SkipCaseActionsParams, now time.Time) error {
	var caseModel structs.Case
	result := tx.Where("id = ?", execution.CaseID).Limit(1).Find(&caseModel)
	if result.Error != nil {
		return fmt.Errorf("get case for skipped action audit: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	return createAuditLogEntry(tx, &structs.AuditLogEntry{
		GuildID:       caseModel.GuildID,
		Source:        structs.AuditSourceSystem,
		Action:        "case_action.skipped",
		ResourceType:  "case_action_execution",
		ResourceID:    execution.ID,
		Result:        structs.AuditResultFailure,
		FailureReason: params.Reason,
		CorrelationID: firstNonEmpty(params.CorrelationID, execution.CorrelationID, caseModel.CorrelationID),
		RequestID:     params.RequestID,
		MetadataJSON: marshalJSONObject(map[string]any{
			"case_id":                caseModel.ID,
			"case_number":            caseModel.CaseNumber,
			"action_type":            execution.ActionType,
			"blocked_after_position": params.AfterPosition,
		}),
	}, now)
}

func (s *Store) ListExecutableCaseIDs(ctx context.Context, limit int) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	if limit <= 0 {
		limit = 100
	}

	now := time.Now().UTC()
	var rows []struct {
		CaseID string
	}
	if err := s.db.WithContext(ctx).
		Model(&structs.CaseActionExecution{}).
		Select("case_id").
		Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", []structs.ActionExecutionStatus{structs.ActionExecutionPending, structs.ActionExecutionRetrying}, now).
		Group("case_id").
		Order("MIN(position) ASC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list executable case ids: %w", err)
	}

	caseIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		caseIDs = append(caseIDs, row.CaseID)
	}
	return caseIDs, nil
}

func appendCaseEvent(tx *gorm.DB, event *structs.CaseEvent, now time.Time) error {
	var caseModel structs.Case
	result := tx.Where("id = ?", event.CaseID).Limit(1).Find(&caseModel)
	if result.Error != nil {
		return fmt.Errorf("get case for event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	event.GuildID = caseModel.GuildID
	if event.Visibility == "" {
		event.Visibility = structs.EventVisibilityStaff
	}
	if event.ActorType == "" {
		event.ActorType = "system"
	}
	if event.MetadataJSON == "" {
		event.MetadataJSON = "{}"
	}
	if err := prepareULIDModel(&event.ULIDModel, now); err != nil {
		return fmt.Errorf("prepare case event model: %w", err)
	}
	if err := tx.Select("*").Create(event).Error; err != nil {
		return fmt.Errorf("create case event: %w", err)
	}
	return nil
}

func updateCaseStatusFromActions(tx *gorm.DB, caseID string, now time.Time) error {
	var executions []structs.CaseActionExecution
	if err := tx.Where("case_id = ?", caseID).Find(&executions).Error; err != nil {
		return fmt.Errorf("list case actions for status: %w", err)
	}
	if len(executions) == 0 {
		return nil
	}

	nextStatus := structs.CaseStatusCompleted
	for _, execution := range executions {
		switch execution.Status {
		case structs.ActionExecutionFailed:
			nextStatus = structs.CaseStatusFailed
		case structs.ActionExecutionPending, structs.ActionExecutionRunning, structs.ActionExecutionRetrying:
			if nextStatus != structs.CaseStatusFailed {
				nextStatus = structs.CaseStatusActionRunning
			}
		}
	}

	updates := map[string]any{
		"status":     nextStatus,
		"updated_at": now,
	}
	if nextStatus == structs.CaseStatusCompleted || nextStatus == structs.CaseStatusFailed {
		updates["resolved_at"] = now
		updates["resolved_by_discord_user_id"] = "system"
	}

	if err := tx.Model(&structs.Case{}).Where("id = ?", caseID).Updates(updates).Error; err != nil {
		return fmt.Errorf("update case status: %w", err)
	}
	return nil
}

func marshalJSONObject(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}
