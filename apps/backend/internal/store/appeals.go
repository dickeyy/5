package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/quackdiscord/bot/internal/quack/idutil"
	"github.com/quackdiscord/bot/internal/quack/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// appealV5Record is the logical 0200 live persistence shape layered over the preserved placeholder table.
type appealV5Record struct {
	ULIDModelRecord
	GuildID                 string             `gorm:"type:char(26);not null;index:idx_appeal_guild_status,priority:1;index:idx_appeal_guild_user,priority:1"`
	CaseID                  *string            `gorm:"type:char(26);uniqueIndex"`
	TargetDiscordUserID     string             `gorm:"size:32;not null;index:idx_appeal_guild_user,priority:2"`
	Status                  model.AppealStatus `gorm:"size:32;not null;default:'pending';index:idx_appeal_guild_status,priority:2"`
	Content                 string             `gorm:"type:text;not null"`
	QuestionSnapshotJSON    string             `gorm:"type:json;not null"`
	AnswersJSON             string             `gorm:"type:json;not null"`
	Version                 uint64             `gorm:"type:bigint unsigned;not null;default:1"`
	DecisionReason          string             `gorm:"type:text"`
	ReviewedByDiscordUserID string             `gorm:"size:32"`
	ReviewedAt              *time.Time         `gorm:"index"`
	ReviewMessageDiscordID  string             `gorm:"size:32"`
	MetadataJSON            string             `gorm:"type:json;not null"`
}

// TableName preserves the appeal table while logical migration 0200 extends it.
func (appealV5Record) TableName() string { return "appeals" }

// appealEventV5Record is the logical 0200 immutable timeline shape.
type appealEventV5Record struct {
	ULIDModelRecord
	AppealID           string `gorm:"type:char(26);not null;index"`
	GuildID            string `gorm:"type:char(26);not null;index"`
	EventType          string `gorm:"size:64;not null;index"`
	ActorDiscordUserID string `gorm:"size:32;index"`
	ActorType          string `gorm:"size:32;not null"`
	Body               string `gorm:"type:text;not null"`
	MetadataJSON       string `gorm:"type:json;not null"`
}

// TableName preserves the appeal event table while logical migration 0200 extends it.
func (appealEventV5Record) TableName() string { return "appeal_events" }

// GetGuildAppealSettings returns a guild's configured future appeal form, or nil to select the product default.
func (s *Store) GetGuildAppealSettings(ctx context.Context, guildID string) (*model.GuildAppealSettings, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var record GuildAppealSettingsRecord
	result := s.db.WithContext(ctx).Where("guild_id = ?", guildID).First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return guildAppealSettingsModel(record), nil
}

// UpdateGuildAppealSettings replaces only the form used by future submissions and appends its audit record atomically.
func (s *Store) UpdateGuildAppealSettings(ctx context.Context, params model.UpdateGuildAppealSettingsParams) (*model.GuildAppealSettings, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	now := time.Now().UTC()
	settings := params.Settings
	if err := prepareULIDModel(&settings.ULIDModel, now); err != nil {
		return nil, err
	}
	record := guildAppealSettingsRecord(settings)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing GuildAppealSettingsRecord
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("guild_id = ?", settings.GuildID).First(&existing)
		if result.Error == nil {
			record.ID = existing.ID
			record.CreatedAt = existing.CreatedAt
			record.UpdatedAt = now
			if err := tx.Select("questions_json", "updated_by_discord_user_id", "updated_at").Updates(&record).Error; err != nil {
				return err
			}
		} else if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			if err := tx.Create(&record).Error; err != nil {
				return err
			}
		} else {
			return result.Error
		}
		audit := params.Audit
		audit.ResourceID = record.ID
		return createAuditLogEntry(tx, &audit, now)
	})
	if err != nil {
		return nil, err
	}
	return guildAppealSettingsModel(record), nil
}

// CreateAppeal inserts one case-unique appeal with its first immutable event, public case history, audit, and staff notification.
func (s *Store) CreateAppeal(ctx context.Context, params model.CreateAppealParams) (*model.Appeal, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	now := time.Now().UTC()
	appeal := params.Appeal
	if err := prepareULIDModel(&appeal.ULIDModel, now); err != nil {
		return nil, err
	}
	event := params.Event
	event.AppealID = appeal.ID
	event.GuildID = appeal.GuildID
	if err := prepareULIDModel(&event.ULIDModel, now); err != nil {
		return nil, err
	}
	notification := params.Notification
	notification.AppealID = appeal.ID
	notification.EventID = event.ID
	notification.GuildID = appeal.GuildID
	if err := prepareULIDModel(&notification.ULIDModel, now); err != nil {
		return nil, err
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if appeal.CaseID == nil || strings.TrimSpace(*appeal.CaseID) == "" {
			return model.ErrAppealCaseIneligible
		}
		var item CaseRecord
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND guild_id = ?", *appeal.CaseID, appeal.GuildID).First(&item)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.ErrAppealCaseIneligible
		}
		if result.Error != nil {
			return result.Error
		}
		if item.TargetDiscordUserID != appeal.TargetDiscordUserID || item.Validity != model.CaseValidityValid || !snapshotAppealable(item.TemplateSnapshotJSON) {
			return model.ErrAppealCaseIneligible
		}
		var existing int64
		if err := tx.Model(&appealV5Record{}).Where("case_id = ?", item.ID).Count(&existing).Error; err != nil {
			return err
		}
		if existing != 0 {
			return model.ErrAppealAlreadyExists
		}
		if err := tx.Create(appealRecord(appeal)).Error; err != nil {
			if isDuplicateError(err) {
				return model.ErrAppealAlreadyExists
			}
			return err
		}
		if err := tx.Create(appealEventRecord(event)).Error; err != nil {
			return err
		}
		caseEvent := params.CaseEvent
		caseEvent.CaseID = item.ID
		caseEvent.GuildID = item.GuildID
		if err := appendCaseEvent(tx, &caseEvent, now); err != nil {
			return err
		}
		if err := tx.Create(appealNotificationRecord(notification)).Error; err != nil {
			return err
		}
		audit := params.Audit
		audit.ResourceID = appeal.ID
		return createAuditLogEntry(tx, &audit, now)
	})
	if err != nil {
		return nil, err
	}
	return &appeal, nil
}

// GetAppealByID returns one appeal without applying caller authorization.
func (s *Store) GetAppealByID(ctx context.Context, appealID string) (*model.Appeal, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var record appealV5Record
	result := s.db.WithContext(ctx).Where("id = ?", appealID).First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return appealModel(record), nil
}

// GetAppealByCaseID returns the only appeal for a case, if any.
func (s *Store) GetAppealByCaseID(ctx context.Context, caseID string) (*model.Appeal, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var record appealV5Record
	result := s.db.WithContext(ctx).Where("case_id = ?", caseID).First(&record)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if result.Error != nil {
		return nil, result.Error
	}
	return appealModel(record), nil
}

// ListAppeals returns stable newest-first staff queue pagination for one guild.
func (s *Store) ListAppeals(ctx context.Context, params model.AppealListParams) (*model.AppealListResult, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	query := s.db.WithContext(ctx).Model(&appealV5Record{}).Where("guild_id = ?", params.GuildID)
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var records []appealV5Record
	if err := query.Order("created_at DESC, id DESC").Limit(params.Limit).Offset(params.Offset).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]model.Appeal, 0, len(records))
	for _, record := range records {
		items = append(items, *appealModel(record))
	}
	return &model.AppealListResult{Appeals: items, Total: total}, nil
}

// ListAppealEvents returns one immutable timeline in creation order.
func (s *Store) ListAppealEvents(ctx context.Context, appealID string) ([]model.AppealEvent, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	var records []appealEventV5Record
	if err := s.db.WithContext(ctx).Where("appeal_id = ?", appealID).Order("created_at ASC, id ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]model.AppealEvent, 0, len(records))
	for _, record := range records {
		items = append(items, appealEventModel(record))
	}
	return items, nil
}

// AppendAppealInformation appends a member response and reopens the same appeal for staff review.
func (s *Store) AppendAppealInformation(ctx context.Context, params model.AppendAppealInformationParams) (*model.Appeal, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	now := time.Now().UTC()
	var updated appealV5Record
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", params.AppealID).First(&updated)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) || updated.TargetDiscordUserID != params.TargetDiscordUserID {
			return model.ErrAppealStateConflict
		}
		if result.Error != nil {
			return result.Error
		}
		if updated.Status != model.AppealStatusNeedsInformation {
			return model.ErrAppealStateConflict
		}
		updated.Status = model.AppealStatusPending
		updated.Version++
		updated.UpdatedAt = now
		result = tx.Model(&appealV5Record{}).Where("id = ? AND version = ?", updated.ID, updated.Version-1).Updates(map[string]any{"status": updated.Status, "version": updated.Version, "updated_at": now})
		if result.Error != nil || result.RowsAffected != 1 {
			return model.ErrAppealStateConflict
		}
		event := params.Event
		event.AppealID = updated.ID
		event.GuildID = updated.GuildID
		event.Body = params.Body
		if err := prepareULIDModel(&event.ULIDModel, now); err != nil {
			return err
		}
		if err := tx.Create(appealEventRecord(event)).Error; err != nil {
			return err
		}
		notification := params.Notification
		notification.AppealID = updated.ID
		notification.EventID = event.ID
		notification.GuildID = updated.GuildID
		if err := prepareULIDModel(&notification.ULIDModel, now); err != nil {
			return err
		}
		if err := tx.Create(appealNotificationRecord(notification)).Error; err != nil {
			return err
		}
		audit := params.Audit
		audit.ResourceID = updated.ID
		return createAuditLogEntry(tx, &audit, now)
	})
	if err != nil {
		return nil, err
	}
	return appealModel(updated), nil
}

// TransitionAppeal applies one staff timeline transition and optionally voids the case in the same transaction.
func (s *Store) TransitionAppeal(ctx context.Context, params model.TransitionAppealParams) (*model.Appeal, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	now := time.Now().UTC()
	var updated appealV5Record
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND guild_id = ?", params.AppealID, params.GuildID).First(&updated)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return model.ErrAppealStateConflict
		}
		if result.Error != nil {
			return result.Error
		}
		if !appealStatusIn(updated.Status, params.AllowedFrom) {
			return model.ErrAppealStateConflict
		}
		if params.VoidCase {
			if updated.CaseID == nil {
				return model.ErrAppealCaseIneligible
			}
			var item CaseRecord
			caseResult := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND guild_id = ?", *updated.CaseID, params.GuildID).First(&item)
			if caseResult.Error != nil || item.Validity != model.CaseValidityValid {
				return model.ErrAppealStateConflict
			}
			caseUpdates := map[string]any{"status": model.CaseValidityVoided, "voided_reason": "Appeal accepted", "voided_by_discord_user_id": params.ActorDiscordUserID, "voided_at": now, "updated_at": now}
			if result := tx.Model(&CaseRecord{}).Where("id = ? AND status = ?", item.ID, model.CaseValidityValid).Updates(caseUpdates); result.Error != nil || result.RowsAffected != 1 {
				return model.ErrAppealStateConflict
			}
			if err := tx.Model(&model.CaseActionExecution{}).Where("case_id = ? AND status IN ?", item.ID, []model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying}).Updates(map[string]any{"status": model.ActionExecutionCancelled, "last_error_code": "case_voided", "last_error": "case was voided before enforcement", "finished_at": now, "next_retry_at": nil}).Error; err != nil {
				return fmt.Errorf("cancel appeal-voided case actions: %w", err)
			}
			if err := tx.Model(&model.CaseNotification{}).Where("case_id = ? AND status IN ?", item.ID, []model.NotificationStatus{model.NotificationPending, model.NotificationPrepared, model.NotificationClaimed}).Updates(map[string]any{"status": model.NotificationFailed, "last_error_code": "case_voided", "last_error": "case was voided before notification", "lease_token": "", "lease_expires_at": nil, "updated_at": now}).Error; err != nil {
				return fmt.Errorf("cancel appeal-voided case notification: %w", err)
			}
			caseEvent := model.CaseEvent{CaseID: item.ID, GuildID: item.GuildID, EventType: model.CaseEventVoided, ActorDiscordUserID: params.ActorDiscordUserID, ActorType: "staff", Visibility: model.EventVisibilityPublic, Body: "Case voided after appeal accepted", MetadataJSON: "{}"}
			if err := appendCaseEvent(tx, &caseEvent, now); err != nil {
				return err
			}
			if params.CaseAudit != nil {
				caseAudit := *params.CaseAudit
				caseAudit.ResourceID = item.ID
				if err := createAuditLogEntry(tx, &caseAudit, now); err != nil {
					return err
				}
			}
		}
		updated.Status = params.To
		updated.DecisionReason = params.Reason
		updated.ReviewedByDiscordUserID = params.ActorDiscordUserID
		updated.ReviewedAt = &now
		updated.Version++
		updated.UpdatedAt = now
		result = tx.Model(&appealV5Record{}).Where("id = ? AND version = ?", updated.ID, updated.Version-1).Updates(map[string]any{
			"status": updated.Status, "decision_reason": updated.DecisionReason,
			"reviewed_by_discord_user_id": updated.ReviewedByDiscordUserID,
			"reviewed_at":                 now, "version": updated.Version, "updated_at": now,
		})
		if result.Error != nil || result.RowsAffected != 1 {
			return model.ErrAppealStateConflict
		}
		event := params.Event
		event.AppealID = updated.ID
		event.GuildID = updated.GuildID
		if err := prepareULIDModel(&event.ULIDModel, now); err != nil {
			return err
		}
		if err := tx.Create(appealEventRecord(event)).Error; err != nil {
			return err
		}
		notification := params.Notification
		notification.AppealID = updated.ID
		notification.EventID = event.ID
		notification.GuildID = updated.GuildID
		if err := prepareULIDModel(&notification.ULIDModel, now); err != nil {
			return err
		}
		if err := tx.Create(appealNotificationRecord(notification)).Error; err != nil {
			return err
		}
		audit := params.AppealAudit
		audit.ResourceID = updated.ID
		return createAuditLogEntry(tx, &audit, now)
	})
	if err != nil {
		return nil, err
	}
	return appealModel(updated), nil
}

// ClaimPendingAppealNotifications atomically leases bounded outbox work so concurrent dispatchers cannot deliver one event twice.
func (s *Store) ClaimPendingAppealNotifications(ctx context.Context, limit int) ([]model.AppealNotification, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	if limit < 1 || limit > 100 {
		return nil, errors.New("appeal notification claim limit is invalid")
	}
	now := time.Now().UTC()
	token, err := idutil.NewULID()
	if err != nil {
		return nil, fmt.Errorf("create appeal notification lease token: %w", err)
	}
	expiresAt := now.Add(2 * time.Minute)
	var records []AppealNotificationRecord
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ? OR (status = ? AND lease_expires_at <= ?)", model.AppealNotificationPending, model.AppealNotificationClaimed, now).
			Order("created_at ASC").Limit(limit).Find(&records)
		if result.Error != nil || len(records) == 0 {
			return result.Error
		}
		ids := make([]string, 0, len(records))
		for index := range records {
			ids = append(ids, records[index].ID)
			records[index].Status = model.AppealNotificationClaimed
			records[index].LeaseToken = token
			records[index].LeaseExpiresAt = &expiresAt
			records[index].UpdatedAt = now
		}
		result = tx.Model(&AppealNotificationRecord{}).Where("id IN ?", ids).Updates(map[string]any{"status": model.AppealNotificationClaimed, "lease_token": token, "lease_expires_at": expiresAt, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(records)) {
			return model.ErrAppealStateConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	items := make([]model.AppealNotification, 0, len(records))
	for _, record := range records {
		items = append(items, appealNotificationModel(record))
	}
	return items, nil
}

// CompleteAppealNotification records delivery without changing the underlying appeal timeline.
func (s *Store) CompleteAppealNotification(ctx context.Context, params model.CompleteAppealNotificationParams) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}
	if params.Status != model.AppealNotificationSent && params.Status != model.AppealNotificationFailed {
		return errors.New("appeal notification completion status is invalid")
	}
	result := s.db.WithContext(ctx).Model(&AppealNotificationRecord{}).Where("id = ? AND status = ? AND lease_token = ?", params.NotificationID, model.AppealNotificationClaimed, params.LeaseToken).Updates(map[string]any{"status": params.Status, "delivery_message_id": params.DeliveryMessageID, "last_error_code": params.ErrorCode, "lease_token": "", "lease_expires_at": nil, "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return model.ErrAppealStateConflict
	}
	return nil
}

func snapshotAppealable(body string) bool {
	var snapshot struct {
		Template struct {
			Appealable bool `json:"appealable"`
		} `json:"template"`
	}
	return json.Unmarshal([]byte(body), &snapshot) == nil && snapshot.Template.Appealable
}

func appealStatusIn(status model.AppealStatus, allowed []model.AppealStatus) bool {
	for _, candidate := range allowed {
		if status == candidate {
			return true
		}
	}
	return false
}

func isDuplicateError(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "duplicate") || strings.Contains(text, "unique constraint")
}

func appealRecord(item model.Appeal) *appealV5Record {
	return &appealV5Record{ULIDModelRecord: ULIDModelRecord{ID: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}, GuildID: item.GuildID, CaseID: item.CaseID, TargetDiscordUserID: item.TargetDiscordUserID, Status: item.Status, Content: item.Content, QuestionSnapshotJSON: item.QuestionSnapshotJSON, AnswersJSON: item.AnswersJSON, Version: item.Version, DecisionReason: item.DecisionReason, ReviewedByDiscordUserID: item.ReviewedByDiscordUserID, ReviewedAt: item.ReviewedAt, ReviewMessageDiscordID: item.ReviewMessageDiscordID, MetadataJSON: item.MetadataJSON}
}

func appealModel(record appealV5Record) *model.Appeal {
	return &model.Appeal{ULIDModel: model.ULIDModel{ID: record.ID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, GuildID: record.GuildID, CaseID: record.CaseID, TargetDiscordUserID: record.TargetDiscordUserID, Status: record.Status, Content: record.Content, QuestionSnapshotJSON: record.QuestionSnapshotJSON, AnswersJSON: record.AnswersJSON, Version: record.Version, DecisionReason: record.DecisionReason, ReviewedByDiscordUserID: record.ReviewedByDiscordUserID, ReviewedAt: record.ReviewedAt, ReviewMessageDiscordID: record.ReviewMessageDiscordID, MetadataJSON: record.MetadataJSON}
}

func appealEventRecord(item model.AppealEvent) *appealEventV5Record {
	return &appealEventV5Record{ULIDModelRecord: ULIDModelRecord{ID: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}, AppealID: item.AppealID, GuildID: item.GuildID, EventType: item.EventType, ActorDiscordUserID: item.ActorDiscordUserID, ActorType: item.ActorType, Body: item.Body, MetadataJSON: item.MetadataJSON}
}

func appealEventModel(record appealEventV5Record) model.AppealEvent {
	return model.AppealEvent{ULIDModel: model.ULIDModel{ID: record.ID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, AppealID: record.AppealID, GuildID: record.GuildID, EventType: record.EventType, ActorDiscordUserID: record.ActorDiscordUserID, ActorType: record.ActorType, Body: record.Body, MetadataJSON: record.MetadataJSON}
}

func guildAppealSettingsRecord(item model.GuildAppealSettings) GuildAppealSettingsRecord {
	return GuildAppealSettingsRecord{ULIDModelRecord: ULIDModelRecord{ID: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}, GuildID: item.GuildID, QuestionsJSON: item.QuestionsJSON, UpdatedByDiscordUserID: item.UpdatedByDiscordUserID}
}

func guildAppealSettingsModel(record GuildAppealSettingsRecord) *model.GuildAppealSettings {
	return &model.GuildAppealSettings{ULIDModel: model.ULIDModel{ID: record.ID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, GuildID: record.GuildID, QuestionsJSON: record.QuestionsJSON, UpdatedByDiscordUserID: record.UpdatedByDiscordUserID}
}

func appealNotificationRecord(item model.AppealNotification) *AppealNotificationRecord {
	return &AppealNotificationRecord{ULIDModelRecord: ULIDModelRecord{ID: item.ID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}, AppealID: item.AppealID, EventID: item.EventID, GuildID: item.GuildID, TargetDiscordUserID: item.TargetDiscordUserID, Audience: item.Audience, Status: item.Status, Body: item.Body, DeliveryMessageID: item.DeliveryMessageID, LastErrorCode: item.LastErrorCode, LeaseToken: item.LeaseToken, LeaseExpiresAt: item.LeaseExpiresAt}
}

func appealNotificationModel(record AppealNotificationRecord) model.AppealNotification {
	return model.AppealNotification{ULIDModel: model.ULIDModel{ID: record.ID, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, AppealID: record.AppealID, EventID: record.EventID, GuildID: record.GuildID, TargetDiscordUserID: record.TargetDiscordUserID, Audience: record.Audience, Status: record.Status, Body: record.Body, DeliveryMessageID: record.DeliveryMessageID, LastErrorCode: record.LastErrorCode, LeaseToken: record.LeaseToken, LeaseExpiresAt: record.LeaseExpiresAt}
}
