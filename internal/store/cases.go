package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// retiredCaseEventTypes are preserved compatibility rows that must not cross the live v5 event boundary.
var retiredCaseEventTypes = []string{"note_added", "note_edited", "note_deleted", "status_changed"}

// CreateCaseParams aliases the core create case params contract so Store satisfies the port without maintaining a second data shape.
type CreateCaseParams = model.CreateCaseParams

// CreatedCase aliases the core created case contract so Store satisfies the port without maintaining a second data shape.
type CreatedCase = model.CreatedCase

// CountTemplateCasesForTargetParams aliases the core count template cases for target params contract so Store satisfies the port without maintaining a second data shape.
type CountTemplateCasesForTargetParams = model.CountTemplateCasesForTargetParams

// ListCasesParams aliases the core list cases params contract so Store satisfies the port without maintaining a second data shape.
type ListCasesParams = model.ListCasesParams

// ListCasesResult aliases the core list cases result contract so Store satisfies the port without maintaining a second data shape.
type ListCasesResult = model.ListCasesResult

// TargetCaseSummary aliases the core target case summary contract so Store satisfies the port without maintaining a second data shape.
type TargetCaseSummary = model.TargetCaseSummary

// ClaimedCaseAction aliases the core claimed case action contract so Store satisfies the port without maintaining a second data shape.
type ClaimedCaseAction = model.ClaimedCaseAction

// ClaimCaseActionParams aliases the core claim case action params contract so Store satisfies the port without maintaining a second data shape.
type ClaimCaseActionParams = model.ClaimCaseActionParams

// CompleteCaseActionParams aliases the core complete case action params contract so Store satisfies the port without maintaining a second data shape.
type CompleteCaseActionParams = model.CompleteCaseActionParams

// SkipCaseActionsParams aliases the core skip case actions params contract so Store satisfies the port without maintaining a second data shape.
type SkipCaseActionsParams = model.SkipCaseActionsParams

// CreateCase creates case while preserving validation, authorization, and persistence invariants.
func (s *Store) CreateCase(ctx context.Context, params CreateCaseParams) (*CreatedCase, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	caseModel := params.Case
	if err := prepareULIDModel(&caseModel.ULIDModel, now); err != nil {
		return nil, fmt.Errorf("prepare case model: %w", err)
	}
	if caseModel.Validity == "" {
		caseModel.Validity = model.CaseValidityValid
	}
	if caseModel.Source == "" {
		caseModel.Source = model.CaseSourceDashboard
	}
	if caseModel.TemplateSnapshotJSON == "" {
		caseModel.TemplateSnapshotJSON = "{}"
	}
	if caseModel.MetadataJSON == "" {
		caseModel.MetadataJSON = "{}"
	}

	event := params.Event
	actionExecutions := make([]model.CaseActionExecution, len(params.ActionExecutions))
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
			event.EventType = model.CaseEventCreated
		}
		if event.ActorType == "" {
			event.ActorType = "staff"
		}
		if event.Visibility == "" {
			event.Visibility = model.EventVisibilityStaff
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
				actionExecutions[i].Status = model.ActionExecutionPending
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

// CountTemplateCasesForTarget encapsulates the count template cases for target rule so callers share one consistent package implementation.
func (s *Store) CountTemplateCasesForTarget(ctx context.Context, params CountTemplateCasesForTargetParams) (int64, error) {
	if s == nil || s.db == nil {
		return 0, errors.New("database not connected")
	}

	query := s.db.WithContext(ctx).
		Model(&model.Case{}).
		Where("guild_id = ?", params.GuildID).
		Where("template_id = ?", params.TemplateID).
		Where("target_discord_user_id = ?", params.TargetDiscordUserID).
		Where("status <> ?", model.CaseValidityVoided)
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count template cases for target: %w", err)
	}

	return count, nil
}

// ListCases returns cases subject to authorization, ordering, and filtering constraints.
func (s *Store) ListCases(ctx context.Context, guildID string) ([]model.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var cases []model.Case
	if err := s.db.WithContext(ctx).Where("guild_id = ?", guildID).Order("case_number ASC").Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list cases: %w", err)
	}

	return cases, nil
}

// ListCasesFiltered returns cases filtered subject to authorization, ordering, and filtering constraints.
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
	if err := filteredCasesQuery(s.db.WithContext(ctx).Model(&model.Case{}), params).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count cases: %w", err)
	}

	var cases []model.Case
	if err := filteredCasesQuery(s.db.WithContext(ctx).Model(&model.Case{}), params).
		Order("case_number DESC").
		Limit(limit).
		Offset(offset).
		Find(&cases).Error; err != nil {
		return nil, fmt.Errorf("list filtered cases: %w", err)
	}

	return &ListCasesResult{Cases: cases, Total: total}, nil
}

// GetCaseByIDOrNumber retrieves case by idor number without exposing the underlying adapter implementation.
func (s *Store) GetCaseByIDOrNumber(ctx context.Context, guildID, caseRef string) (*model.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	query := s.db.WithContext(ctx).Where("guild_id = ?", guildID)
	if caseNumber, err := strconv.ParseUint(caseRef, 10, 64); err == nil && strconv.FormatUint(caseNumber, 10) == caseRef {
		query = query.Where("case_number = ?", caseNumber)
	} else {
		query = query.Where("id = ?", caseRef)
	}

	var caseModel model.Case
	if err := query.First(&caseModel).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get case by id or number: %w", err)
	}

	return &caseModel, nil
}

// TargetCaseSummary encapsulates the target case summary rule so callers share one consistent package implementation.
func (s *Store) TargetCaseSummary(ctx context.Context, guildID, targetDiscordUserID string) (*TargetCaseSummary, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	summary := &TargetCaseSummary{
		ByValidity: map[model.CaseValidity]int64{},
		ByTemplate: map[string]int64{},
	}

	targetCases := func() *gorm.DB {
		return s.db.WithContext(ctx).Model(&model.Case{}).
			Where("guild_id = ?", guildID).
			Where("target_discord_user_id = ?", targetDiscordUserID)
	}
	if err := targetCases().Count(&summary.Total).Error; err != nil {
		return nil, fmt.Errorf("count target cases: %w", err)
	}

	var validityRows []struct {
		Validity model.CaseValidity `gorm:"column:status"`
		Count    int64
	}
	if err := targetCases().Select("status, COUNT(*) AS count").Group("status").Scan(&validityRows).Error; err != nil {
		return nil, fmt.Errorf("count target cases by validity: %w", err)
	}
	for _, row := range validityRows {
		summary.ByValidity[row.Validity] = row.Count
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

// ListCaseEvents returns case events subject to authorization, ordering, and filtering constraints.
func (s *Store) ListCaseEvents(ctx context.Context, caseID string) ([]model.CaseEvent, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var events []model.CaseEvent
	if err := s.db.WithContext(ctx).
		Where("case_id = ?", caseID).
		Where("event_type NOT IN ?", retiredCaseEventTypes).
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list case events: %w", err)
	}

	return events, nil
}

// ListCaseActionExecutions returns case action executions subject to authorization, ordering, and filtering constraints.
func (s *Store) ListCaseActionExecutions(ctx context.Context, caseID string) ([]model.CaseActionExecution, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var executions []model.CaseActionExecution
	if err := s.db.WithContext(ctx).Where("case_id = ?", caseID).Order("position ASC").Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("list case action executions: %w", err)
	}

	return executions, nil
}

// ListCaseActionAttempts returns case action attempts subject to authorization, ordering, and filtering constraints.
func (s *Store) ListCaseActionAttempts(ctx context.Context, executionIDs []string) ([]model.CaseActionAttempt, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	if len(executionIDs) == 0 {
		return nil, nil
	}

	var attempts []model.CaseActionAttempt
	if err := s.db.WithContext(ctx).
		Where("execution_id IN ?", executionIDs).
		Order("execution_id ASC, attempt_number ASC").
		Find(&attempts).Error; err != nil {
		return nil, fmt.Errorf("list case action attempts: %w", err)
	}

	return attempts, nil
}

// filteredCasesQuery encapsulates the filtered cases query rule so callers share one consistent package implementation.
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
	if params.Validity != "" {
		query = query.Where("status = ?", params.Validity)
	}
	return query
}

// firstNonEmpty encapsulates the first non empty rule so callers share one consistent package implementation.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// nextCaseNumber encapsulates the next case number rule so callers share one consistent package implementation.
func nextCaseNumber(tx *gorm.DB, guildID string) (uint64, error) {
	var latest model.Case
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

// ClaimNextCaseAction atomically claims next case action so concurrent workers cannot execute it twice.
func (s *Store) ClaimNextCaseAction(ctx context.Context, params ClaimCaseActionParams) (*ClaimedCaseAction, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	var claimed *ClaimedCaseAction
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var caseModel model.Case
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
		if err := tx.Model(&model.CaseActionExecution{}).
			Where("case_id = ? AND status = ?", params.CaseID, model.ActionExecutionRunning).
			Count(&running).Error; err != nil {
			return fmt.Errorf("count running case actions: %w", err)
		}
		if running > 0 {
			return nil
		}

		var execution model.CaseActionExecution
		result = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("case_id = ? AND status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)",
				params.CaseID,
				[]model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying},
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

		execution.Status = model.ActionExecutionRunning
		execution.AttemptCount++
		execution.StartedAt = &now
		execution.FinishedAt = nil
		execution.NextRetryAt = nil
		execution.UpdatedAt = now
		if err := tx.Select("*").Save(&execution).Error; err != nil {
			return fmt.Errorf("mark case action running: %w", err)
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

// CompleteCaseAction persists the terminal or retry outcome for case action.
func (s *Store) CompleteCaseAction(ctx context.Context, params CompleteCaseActionParams) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var execution model.CaseActionExecution
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
		attempt := model.CaseActionAttempt{
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
		if params.ExecutionStatus == model.ActionExecutionRetrying {
			execution.FinishedAt = nil
		}
		if err := tx.Select("*").Save(&execution).Error; err != nil {
			return fmt.Errorf("update case action execution: %w", err)
		}

		if params.EventType != "" {
			event := model.CaseEvent{
				CaseID:       execution.CaseID,
				EventType:    params.EventType,
				ActorType:    "system",
				Visibility:   model.EventVisibilityStaff,
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

		return nil
	})
}

// SkipCaseActions marks case actions as skipped when policy prevents further execution.
func (s *Store) SkipCaseActions(ctx context.Context, params SkipCaseActionsParams) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var executions []model.CaseActionExecution
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("case_id = ? AND position > ? AND status IN ?", params.CaseID, params.AfterPosition, []model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying}).
			Order("position ASC").
			Find(&executions).Error; err != nil {
			return fmt.Errorf("list case actions to skip: %w", err)
		}

		for i := range executions {
			executions[i].Status = model.ActionExecutionSkipped
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

		return nil
	})
}

// createCaseActionAudit creates case action audit while preserving validation, authorization, and persistence invariants.
func createCaseActionAudit(tx *gorm.DB, execution model.CaseActionExecution, params CompleteCaseActionParams, now time.Time) error {
	var caseModel model.Case
	result := tx.Where("id = ?", execution.CaseID).Limit(1).Find(&caseModel)
	if result.Error != nil {
		return fmt.Errorf("get case for action audit: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	action := "case_action.succeeded"
	resultValue := model.AuditResultSuccess
	switch params.ExecutionStatus {
	case model.ActionExecutionRetrying:
		action = "case_action.retrying"
		resultValue = model.AuditResultFailure
	case model.ActionExecutionFailed:
		action = "case_action.failed"
		resultValue = model.AuditResultFailure
	}

	return createAuditLogEntry(tx, &model.AuditLogEntry{
		GuildID:       caseModel.GuildID,
		Source:        model.AuditSourceSystem,
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
			"retrying":       params.ExecutionStatus == model.ActionExecutionRetrying,
		}),
	}, now)
}

// createSkippedCaseActionAudit creates skipped case action audit while preserving validation, authorization, and persistence invariants.
func createSkippedCaseActionAudit(tx *gorm.DB, execution model.CaseActionExecution, params SkipCaseActionsParams, now time.Time) error {
	var caseModel model.Case
	result := tx.Where("id = ?", execution.CaseID).Limit(1).Find(&caseModel)
	if result.Error != nil {
		return fmt.Errorf("get case for skipped action audit: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	return createAuditLogEntry(tx, &model.AuditLogEntry{
		GuildID:       caseModel.GuildID,
		Source:        model.AuditSourceSystem,
		Action:        "case_action.skipped",
		ResourceType:  "case_action_execution",
		ResourceID:    execution.ID,
		Result:        model.AuditResultFailure,
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

// ListExecutableCaseIDs returns executable case ids subject to authorization, ordering, and filtering constraints.
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
		Model(&model.CaseActionExecution{}).
		Select("case_id").
		Where("status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", []model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying}, now).
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

// appendCaseEvent encapsulates the append case event rule so callers share one consistent package implementation.
func appendCaseEvent(tx *gorm.DB, event *model.CaseEvent, now time.Time) error {
	var caseModel model.Case
	result := tx.Where("id = ?", event.CaseID).Limit(1).Find(&caseModel)
	if result.Error != nil {
		return fmt.Errorf("get case for event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}

	event.GuildID = caseModel.GuildID
	if event.Visibility == "" {
		event.Visibility = model.EventVisibilityStaff
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

// marshalJSONObject serializes marshal jsonobject into its stable external representation.
func marshalJSONObject(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(body)
}
