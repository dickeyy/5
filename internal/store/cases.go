package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/quackdiscord/bot/internal/quack/idutil"
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
	if caseModel.ContextValuesJSON == "" {
		caseModel.ContextValuesJSON = "[]"
	}

	event := params.Event
	actionExecutions := make([]model.CaseActionExecution, len(params.ActionExecutions))
	copy(actionExecutions, params.ActionExecutions)
	evidence := make([]model.CaseEvidenceSnapshot, len(params.Evidence))
	copy(evidence, params.Evidence)
	attachments := make([]model.CaseEvidenceAttachment, len(params.Attachments))
	copy(attachments, params.Attachments)
	var notification *model.CaseNotification

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

		for i := range evidence {
			evidence[i].CaseID = caseModel.ID
			evidence[i].GuildID = caseModel.GuildID
			if evidence[i].EmbedsJSON == "" {
				evidence[i].EmbedsJSON = "[]"
			}
			if err := prepareULIDModel(&evidence[i].ULIDModel, now); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&evidence[i]).Error; err != nil {
				return fmt.Errorf("create case evidence: %w", err)
			}
		}
		for i := range attachments {
			if attachments[i].EvidenceID == "" {
				return errors.New("evidence attachment has no snapshot")
			}
			if err := prepareULIDModel(&attachments[i].ULIDModel, now); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&attachments[i]).Error; err != nil {
				return fmt.Errorf("create case evidence attachment: %w", err)
			}
		}
		if params.Notification != nil {
			copyValue := *params.Notification
			copyValue.CaseID = caseModel.ID
			if copyValue.Status == "" {
				copyValue.Status = model.NotificationPending
			}
			if err := prepareULIDModel(&copyValue.ULIDModel, now); err != nil {
				return err
			}
			if err := tx.Select("*").Create(&copyValue).Error; err != nil {
				return fmt.Errorf("create case notification: %w", err)
			}
			notification = &copyValue
		}
		if caseModel.ReplacesCaseID != nil {
			result := tx.Model(&model.Case{}).Where("id = ? AND guild_id = ? AND status = ? AND replacement_case_id IS NULL", *caseModel.ReplacesCaseID, caseModel.GuildID, model.CaseValidityVoided).Update("replacement_case_id", caseModel.ID)
			if result.Error != nil {
				return fmt.Errorf("link replacement case: %w", result.Error)
			}
			if result.RowsAffected != 1 {
				return errors.New("replacement case is no longer available")
			}
		}

		if params.Audit != nil {
			audit := *params.Audit
			audit.ResourceID = caseModel.ID
			if err := createAuditLogEntry(tx, &audit, now); err != nil {
				return err
			}
		}
		for i := range params.AdditionalAudits {
			entry := params.AdditionalAudits[i]
			if entry.ResourceID == "" || entry.ResourceID == "unknown" {
				entry.ResourceID = caseModel.ID
			}
			if err := createAuditLogEntry(tx, &entry, now); err != nil {
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
		Evidence:         evidence, Attachments: attachments, Notification: notification,
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
	query = query.Where("source <> ?", model.CaseSourceV4Import)
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

// GetCaseByID retrieves a case without requiring current guild membership, for member-owned access checks performed by the service.
func (s *Store) GetCaseByID(ctx context.Context, caseID string) (*model.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var item model.Case
	result := s.db.WithContext(ctx).Where("id = ?", caseID).First(&item)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, fmt.Errorf("get case by id: %w", result.Error)
	}
	return &item, nil
}

// GetCaseByIdempotencyKey retrieves the durable result of an externally retried case request.
func (s *Store) GetCaseByIdempotencyKey(ctx context.Context, guildID, key string) (*model.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var item model.Case
	result := s.db.WithContext(ctx).Where("guild_id = ? AND idempotency_key = ?", guildID, key).First(&item)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &item, nil
}

// VoidCase atomically changes only case validity and appends immutable correction history.
func (s *Store) VoidCase(ctx context.Context, params model.VoidCaseParams) (*model.Case, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	now := time.Now().UTC()
	var item model.Case
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND guild_id = ?", params.CaseID, params.GuildID).First(&item)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if result.Error != nil {
			return result.Error
		}
		if item.Validity == model.CaseValidityVoided {
			if item.VoidedReason == params.Reason {
				return nil
			}
			return errors.New("case is already voided with a different reason")
		}
		item.Validity = model.CaseValidityVoided
		item.VoidedReason = params.Reason
		item.VoidedByDiscordUserID = params.ActorDiscordUserID
		item.VoidedAt = &now
		item.ReplacementCaseID = params.ReplacementCaseID
		item.UpdatedAt = now
		if err := tx.Select("*").Save(&item).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.CaseActionExecution{}).Where("case_id = ? AND status IN ?", item.ID, []model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying}).Updates(map[string]any{"status": model.ActionExecutionCancelled, "last_error_code": "case_voided", "last_error": "case was voided before enforcement", "finished_at": now, "next_retry_at": nil}).Error; err != nil {
			return fmt.Errorf("cancel voided case actions: %w", err)
		}
		if err := tx.Model(&model.CaseNotification{}).Where("case_id = ? AND status IN ?", item.ID, []model.NotificationStatus{model.NotificationPending, model.NotificationPrepared, model.NotificationClaimed}).Updates(map[string]any{"status": model.NotificationFailed, "last_error_code": "case_voided", "last_error": "case was voided before notification", "lease_token": "", "lease_expires_at": nil, "updated_at": now}).Error; err != nil {
			return fmt.Errorf("cancel voided case notification: %w", err)
		}
		event := model.CaseEvent{CaseID: item.ID, EventType: model.CaseEventVoided, ActorDiscordUserID: params.ActorDiscordUserID, ActorType: "staff", Visibility: model.EventVisibilityPublic, Body: "Case voided", MetadataJSON: marshalJSONObject(map[string]any{"reason": params.Reason, "replacement_case_id": params.ReplacementCaseID})}
		if err := appendCaseEvent(tx, &event, now); err != nil {
			return err
		}
		if params.Audit != nil {
			audit := *params.Audit
			audit.ResourceID = item.ID
			if err := createAuditLogEntry(tx, &audit, now); err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
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

// GetCaseActionExecution retrieves one guild-scoped action for staff recovery controls.
func (s *Store) GetCaseActionExecution(ctx context.Context, guildID, executionID string) (*model.CaseActionExecution, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var item model.CaseActionExecution
	result := s.db.WithContext(ctx).Where("id = ? AND case_id IN (SELECT id FROM cases WHERE guild_id = ?)", executionID, guildID).First(&item)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return &item, nil
}

// ListCaseEvidence returns immutable snapshots and attachment metadata for one case.
func (s *Store) ListCaseEvidence(ctx context.Context, caseID string) ([]model.CaseEvidenceSnapshot, []model.CaseEvidenceAttachment, error) {
	if s == nil || s.db == nil {
		return nil, nil, errors.New("database not connected")
	}
	var snapshots []model.CaseEvidenceSnapshot
	if err := s.db.WithContext(ctx).Where("case_id = ?", caseID).Order("created_at ASC").Find(&snapshots).Error; err != nil {
		return nil, nil, fmt.Errorf("list case evidence: %w", err)
	}
	ids := make([]string, 0, len(snapshots))
	for _, item := range snapshots {
		ids = append(ids, item.ID)
	}
	var attachments []model.CaseEvidenceAttachment
	if len(ids) > 0 {
		if err := s.db.WithContext(ctx).Where("evidence_id IN ?", ids).Order("created_at ASC").Find(&attachments).Error; err != nil {
			return nil, nil, fmt.Errorf("list evidence attachments: %w", err)
		}
	}
	return snapshots, attachments, nil
}

// GetCaseNotification returns the one notification record when the selected level enabled member delivery.
func (s *Store) GetCaseNotification(ctx context.Context, caseID string) (*model.CaseNotification, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var item model.CaseNotification
	result := s.db.WithContext(ctx).Where("case_id = ?", caseID).First(&item)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, fmt.Errorf("get case notification: %w", result.Error)
	}
	return &item, nil
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
	if params.CaseNumber != "" {
		query = query.Where("case_number = ?", params.CaseNumber)
	}
	if params.CreatedAfter != "" {
		query = query.Where("created_at >= ?", params.CreatedAfter)
	}
	if params.CreatedBefore != "" {
		query = query.Where("created_at <= ?", params.CreatedBefore)
	}
	if params.ActionResult != "" {
		query = query.Where("EXISTS (SELECT 1 FROM case_action_executions cae WHERE cae.case_id = cases.id AND cae.status = ?)", params.ActionResult)
	}
	if params.AppealStatus != "" {
		query = query.Where("EXISTS (SELECT 1 FROM appeals a WHERE a.case_id = cases.id AND a.status = ?)", params.AppealStatus)
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
			Where("case_id = ? AND status = ? AND lease_expires_at > ?", params.CaseID, model.ActionExecutionRunning, now).
			Count(&running).Error; err != nil {
			return fmt.Errorf("count running case actions: %w", err)
		}
		if running > 0 {
			return nil
		}

		var execution model.CaseActionExecution
		result = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("case_id = ? AND ((status IN ? AND (next_retry_at IS NULL OR next_retry_at <= ?)) OR (status = ? AND lease_expires_at <= ?))", params.CaseID, []model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying}, now, model.ActionExecutionRunning, now).
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
		if execution.AttemptCount > 0 {
			var prior model.CaseActionAttempt
			priorResult := tx.Where("execution_id = ? AND attempt_number = ? AND status = ?", execution.ID, execution.AttemptCount, model.ActionAttemptRunning).First(&prior)
			if priorResult.Error == nil {
				prior.Status = model.ActionAttemptFailed
				prior.FinishedAt = &now
				prior.DurationMS = now.Sub(prior.StartedAt).Milliseconds()
				prior.ErrorCode = "lease_expired"
				prior.ErrorMessage = "worker lease expired before completion"
				prior.UpdatedAt = now
				if err := tx.Select("*").Save(&prior).Error; err != nil {
					return fmt.Errorf("close expired action attempt: %w", err)
				}
				if err := createAuditLogEntry(tx, &model.AuditLogEntry{GuildID: caseModel.GuildID, Source: model.AuditSourceSystem, Action: string(model.AuditActionActionRecovered), ResourceType: "case_action_execution", ResourceID: execution.ID, Result: model.AuditResultFailure, FailureReason: "lease_expired", CorrelationID: firstNonEmpty(execution.CorrelationID, caseModel.CorrelationID), MetadataJSON: marshalJSONObject(map[string]any{"case_id": caseModel.ID, "attempt_number": prior.AttemptNumber, "recovery": "lease_reclaimed"})}, now); err != nil {
					return err
				}
			} else if !errors.Is(priorResult.Error, gorm.ErrRecordNotFound) {
				return priorResult.Error
			}
		}
		execution.AttemptCount++
		execution.StartedAt = &now
		execution.FinishedAt = nil
		execution.NextRetryAt = nil
		leaseToken, err := idutil.NewULID()
		if err != nil {
			return fmt.Errorf("create action lease token: %w", err)
		}
		leaseExpiry := now.Add(2 * time.Minute)
		execution.LeaseToken = leaseToken
		execution.LeaseExpiresAt = &leaseExpiry
		execution.UpdatedAt = now
		if err := tx.Select("*").Save(&execution).Error; err != nil {
			return fmt.Errorf("mark case action running: %w", err)
		}
		attempt := model.CaseActionAttempt{ExecutionID: execution.ID, AttemptNumber: execution.AttemptCount, Status: model.ActionAttemptRunning, WorkerID: params.WorkerID, StartedAt: now, RequestPayloadJSON: "{}", ResponsePayloadJSON: "{}"}
		if err := prepareULIDModel(&attempt.ULIDModel, now); err != nil {
			return err
		}
		if err := tx.Select("*").Create(&attempt).Error; err != nil {
			return fmt.Errorf("create running action attempt: %w", err)
		}
		if err := createAuditLogEntry(tx, &model.AuditLogEntry{GuildID: caseModel.GuildID, Source: model.AuditSourceSystem, Action: string(model.AuditActionActionAttempt), ResourceType: "case_action_execution", ResourceID: execution.ID, Result: model.AuditResultSuccess, CorrelationID: firstNonEmpty(execution.CorrelationID, caseModel.CorrelationID), MetadataJSON: marshalJSONObject(map[string]any{"case_id": caseModel.ID, "attempt_number": attempt.AttemptNumber, "status": attempt.Status})}, now); err != nil {
			return err
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
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", params.ExecutionID)
		if params.LeaseToken != "" {
			query = query.Where("lease_token = ?", params.LeaseToken)
		}
		result := query.
			Limit(1).
			Find(&execution)
		if result.Error != nil {
			return fmt.Errorf("get case action execution: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			if params.LeaseToken != "" {
				return errors.New("case action lease is stale")
			}
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
		attemptNumber := params.AttemptNumber
		if attemptNumber == 0 {
			attemptNumber = execution.AttemptCount
		}
		var attempt model.CaseActionAttempt
		attemptResult := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("execution_id = ? AND attempt_number = ?", execution.ID, attemptNumber).First(&attempt)
		if errors.Is(attemptResult.Error, gorm.ErrRecordNotFound) {
			attempt = model.CaseActionAttempt{ExecutionID: execution.ID, AttemptNumber: attemptNumber, StartedAt: startedAt, WorkerID: params.WorkerID}
			if err := prepareULIDModel(&attempt.ULIDModel, now); err != nil {
				return err
			}
		} else if attemptResult.Error != nil {
			return attemptResult.Error
		}
		attempt.Status = params.AttemptStatus
		attempt.FinishedAt = &now
		attempt.DurationMS = now.Sub(attempt.StartedAt).Milliseconds()
		attempt.ErrorCode = params.ErrorCode
		attempt.ErrorMessage = params.ErrorMessage
		attempt.RequestPayloadJSON = params.RequestPayloadJSON
		attempt.ResponsePayloadJSON = params.ResponsePayloadJSON
		attempt.UpdatedAt = now
		if err := tx.Select("*").Save(&attempt).Error; err != nil {
			return fmt.Errorf("complete case action attempt: %w", err)
		}

		execution.Status = params.ExecutionStatus
		execution.LastErrorCode = params.ErrorCode
		execution.LastError = params.ErrorMessage
		execution.FinishedAt = &now
		execution.NextRetryAt = params.NextRetryAt
		execution.LeaseToken = ""
		execution.LeaseExpiresAt = nil
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
				Visibility:   model.EventVisibilityPublic,
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

// ListExecutableCaseIDs returns a bounded guild-fair executable-case batch.
// Every active guild receives one slot before a busy guild receives another,
// and the cursor rotates across calls when the batch is smaller than the guild
// count. ClaimNextCaseAction remains the authoritative transactional fence.
func (s *Store) ListExecutableCaseIDs(ctx context.Context, limit int) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	if limit <= 0 {
		limit = 100
	}

	s.executableMu.Lock()
	defer s.executableMu.Unlock()

	now := time.Now().UTC()
	var rows []struct {
		CaseID  string
		GuildID string
	}
	query := `
WITH executable_cases AS (
    SELECT e.case_id,
           c.guild_id,
           MIN(e.position) AS first_position,
           MIN(COALESCE(e.next_retry_at, e.lease_expires_at, e.created_at)) AS ready_at
      FROM case_action_executions AS e
      JOIN cases AS c ON c.id = e.case_id
     WHERE ((e.status IN (?, ?) AND (e.next_retry_at IS NULL OR e.next_retry_at <= ?))
            OR (e.status = ? AND e.lease_expires_at <= ?))
     GROUP BY e.case_id, c.guild_id
), ranked_cases AS (
    SELECT case_id,
           guild_id,
           first_position,
           ready_at,
           ROW_NUMBER() OVER (
               PARTITION BY guild_id
               ORDER BY first_position ASC, ready_at ASC, case_id ASC
           ) AS guild_rank
      FROM executable_cases
)
SELECT case_id, guild_id
  FROM ranked_cases
 ORDER BY guild_rank ASC,
          CASE WHEN guild_id > ? THEN 0 ELSE 1 END ASC,
          guild_id ASC,
          first_position ASC,
          ready_at ASC,
          case_id ASC
 LIMIT ?`
	if err := s.db.WithContext(ctx).Raw(query,
		model.ActionExecutionPending,
		model.ActionExecutionRetrying,
		now,
		model.ActionExecutionRunning,
		now,
		s.executableGuildCursor,
		limit,
	).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list executable case ids: %w", err)
	}

	caseIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		caseIDs = append(caseIDs, row.CaseID)
	}
	if len(rows) > 0 {
		s.executableGuildCursor = rows[len(rows)-1].GuildID
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
