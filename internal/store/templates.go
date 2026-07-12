package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
	"gorm.io/gorm"
)

// ExpandedCaseTemplate aliases the core expanded case template contract so Store satisfies the port without maintaining a second data shape.
type ExpandedCaseTemplate = model.ExpandedCaseTemplate

// ExpandedCaseTemplateLevel aliases the core expanded case template level contract so Store satisfies the port without maintaining a second data shape.
type ExpandedCaseTemplateLevel = model.ExpandedCaseTemplateLevel

// CreateCaseTemplateParams aliases the core create case template params contract so Store satisfies the port without maintaining a second data shape.
type CreateCaseTemplateParams = model.CreateCaseTemplateParams

// UpdateCaseTemplateParams aliases the core update case template params contract so Store satisfies the port without maintaining a second data shape.
type UpdateCaseTemplateParams = model.UpdateCaseTemplateParams

// ListAuditLogEntriesParams aliases the core list audit log entries params contract so Store satisfies the port without maintaining a second data shape.
type ListAuditLogEntriesParams = model.ListAuditLogEntriesParams

// ListAuditLogEntriesResult aliases the core list audit log entries result contract so Store satisfies the port without maintaining a second data shape.
type ListAuditLogEntriesResult = model.ListAuditLogEntriesResult

// CreateCaseTemplate creates case template while preserving validation, authorization, and persistence invariants.
func (s *Store) CreateCaseTemplate(ctx context.Context, params CreateCaseTemplateParams) (*ExpandedCaseTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	template := params.Template
	if err := prepareULIDModel(&template.ULIDModel, now); err != nil {
		return nil, fmt.Errorf("prepare case template model: %w", err)
	}
	template.Version = 1
	templateRecord := caseTemplateRecordFromModel(template)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("*").Create(&templateRecord).Error; err != nil {
			return fmt.Errorf("create case template: %w", err)
		}

		if err := createTemplateLevels(tx, template.ID, params.Levels, now); err != nil {
			return err
		}

		if params.Audit != nil {
			audit := *params.Audit
			audit.ResourceID = template.ID
			if err := createAuditLogEntry(tx, &audit, now); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.GetCaseTemplateExpanded(ctx, template.GuildID, template.ID)
}

// ListCaseTemplates returns case templates subject to authorization, ordering, and filtering constraints.
func (s *Store) ListCaseTemplates(ctx context.Context, guildID string) ([]ExpandedCaseTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var records []CaseTemplateRecord
	if err := s.db.WithContext(ctx).Unscoped().
		Where("guild_id = ? AND archived_at IS NULL", guildID).
		Order("slug ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list case templates: %w", err)
	}

	expanded := make([]ExpandedCaseTemplate, 0, len(records))
	for _, record := range records {
		template := caseTemplateModelFromRecord(record)
		item, err := s.GetCaseTemplateExpanded(ctx, guildID, template.ID)
		if err != nil {
			return nil, err
		}
		if item != nil {
			expanded = append(expanded, *item)
		}
	}

	return expanded, nil
}

// GetCaseTemplateExpanded retrieves case template expanded without exposing the underlying adapter implementation.
func (s *Store) GetCaseTemplateExpanded(ctx context.Context, guildID, templateID string) (*ExpandedCaseTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	return getCaseTemplateExpanded(s.db.WithContext(ctx), guildID, templateID)
}

// GetCaseTemplateBySlug retrieves case template by slug without exposing the underlying adapter implementation.
func (s *Store) GetCaseTemplateBySlug(ctx context.Context, guildID, slug string) (*model.CaseTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var record CaseTemplateRecord
	if err := s.db.WithContext(ctx).Unscoped().Where("guild_id = ? AND slug = ?", guildID, slug).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get case template by slug: %w", err)
	}

	template := caseTemplateModelFromRecord(record)
	return &template, nil
}

// UpdateCaseTemplate updates case template while retaining validation, compatibility, and audit requirements.
func (s *Store) UpdateCaseTemplate(ctx context.Context, params UpdateCaseTemplateParams) (*ExpandedCaseTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	var record CaseTemplateRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("id = ? AND guild_id = ?", params.TemplateID, params.GuildID).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return fmt.Errorf("get case template for update: %w", err)
		}

		record.Slug = params.Template.Slug
		record.Name = params.Template.Name
		record.Description = params.Template.Description
		record.ReasonTemplate = params.Template.ReasonTemplate
		record.Appealable = params.Template.Appealable
		record.UpdatedByDiscordUserID = params.Template.UpdatedByDiscordUserID
		record.Version++
		record.UpdatedAt = now

		if err := tx.Save(&record).Error; err != nil {
			return fmt.Errorf("update case template: %w", err)
		}

		levelIDs := tx.Model(&CaseTemplateLevelRecord{}).Select("id").Where("template_id = ?", record.ID)
		if err := tx.Where("level_id IN (?)", levelIDs).Delete(&CaseTemplateLevelActionRecord{}).Error; err != nil {
			return fmt.Errorf("replace case template level actions: %w", err)
		}
		if err := tx.Where("template_id = ?", record.ID).Delete(&CaseTemplateLevelRecord{}).Error; err != nil {
			return fmt.Errorf("replace case template levels: %w", err)
		}
		if err := createTemplateLevels(tx, record.ID, params.Levels, now); err != nil {
			return err
		}

		if params.Audit != nil {
			audit := *params.Audit
			audit.ResourceID = record.ID
			if err := createAuditLogEntry(tx, &audit, now); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return s.GetCaseTemplateExpanded(ctx, params.GuildID, record.ID)
}

// ArchiveCaseTemplate archives case template without deleting historical moderation references.
func (s *Store) ArchiveCaseTemplate(ctx context.Context, guildID, templateID string, audit *model.AuditLogEntry) (*ExpandedCaseTemplate, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	now := time.Now().UTC()
	var record CaseTemplateRecord
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("id = ? AND guild_id = ?", templateID, guildID).First(&record).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return gorm.ErrRecordNotFound
			}
			return fmt.Errorf("get case template for archive: %w", err)
		}

		record.ArchivedAt = &now
		record.UpdatedAt = now
		if err := tx.Save(&record).Error; err != nil {
			return fmt.Errorf("archive case template: %w", err)
		}

		if audit != nil {
			audit := *audit
			audit.ResourceID = record.ID
			if err := createAuditLogEntry(tx, &audit, now); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return s.GetCaseTemplateExpanded(ctx, guildID, record.ID)
}

// CreateAuditLogEntry creates audit log entry while preserving validation, authorization, and persistence invariants.
func (s *Store) CreateAuditLogEntry(ctx context.Context, entry *model.AuditLogEntry) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	return createAuditLogEntry(s.db.WithContext(ctx), entry, time.Now().UTC())
}

// ListAuditLogEntries returns audit log entries subject to authorization, ordering, and filtering constraints.
func (s *Store) ListAuditLogEntries(ctx context.Context, guildID string) ([]model.AuditLogEntry, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}

	var entries []model.AuditLogEntry
	if err := s.db.WithContext(ctx).Where("guild_id = ?", guildID).Order("created_at ASC").Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("list audit log entries: %w", err)
	}

	return entries, nil
}

// ListAuditLogEntriesFiltered returns audit log entries filtered subject to authorization, ordering, and filtering constraints.
func (s *Store) ListAuditLogEntriesFiltered(ctx context.Context, params ListAuditLogEntriesParams) (*ListAuditLogEntriesResult, error) {
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
	if err := filteredAuditQuery(s.db.WithContext(ctx).Model(&model.AuditLogEntry{}), params).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count audit log entries: %w", err)
	}

	var entries []model.AuditLogEntry
	if err := filteredAuditQuery(s.db.WithContext(ctx).Model(&model.AuditLogEntry{}), params).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&entries).Error; err != nil {
		return nil, fmt.Errorf("list filtered audit log entries: %w", err)
	}

	return &ListAuditLogEntriesResult{Entries: entries, Total: total}, nil
}

// filteredAuditQuery encapsulates the filtered audit query rule so callers share one consistent package implementation.
func filteredAuditQuery(query *gorm.DB, params ListAuditLogEntriesParams) *gorm.DB {
	query = query.Where("guild_id = ?", params.GuildID)
	if params.ActorDiscordUserID != "" {
		query = query.Where("actor_discord_user_id = ?", params.ActorDiscordUserID)
	}
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}
	if params.ResourceType != "" {
		query = query.Where("resource_type = ?", params.ResourceType)
	}
	if params.ResourceID != "" {
		query = query.Where("resource_id = ?", params.ResourceID)
	}
	if params.Result != "" {
		query = query.Where("result = ?", params.Result)
	}
	return query
}

// getCaseTemplateExpanded retrieves case template expanded without exposing the underlying adapter implementation.
func getCaseTemplateExpanded(db *gorm.DB, guildID, templateID string) (*ExpandedCaseTemplate, error) {
	var templateRecord CaseTemplateRecord
	if err := db.Unscoped().Where("id = ? AND guild_id = ?", templateID, guildID).First(&templateRecord).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get case template: %w", err)
	}

	var compatibility migration0002TemplateCompatibility
	compatibilityResult := db.Where("template_id = ?", templateRecord.ID).Limit(1).Find(&compatibility)
	if compatibilityResult.Error != nil {
		return nil, fmt.Errorf("get case template compatibility state: %w", compatibilityResult.Error)
	}
	if compatibilityResult.RowsAffected > 0 {
		return nil, &model.TemplateCompatibilityReviewError{
			TemplateID: templateRecord.ID,
			Reason:     compatibility.Reason,
		}
	}

	template := caseTemplateModelFromRecord(templateRecord)
	var levelRecords []CaseTemplateLevelRecord
	if err := db.Where("template_id = ?", template.ID).Order("position ASC").Find(&levelRecords).Error; err != nil {
		return nil, fmt.Errorf("get case template levels: %w", err)
	}

	expandedLevels := make([]ExpandedCaseTemplateLevel, 0, len(levelRecords))
	for _, levelRecord := range levelRecords {
		level := caseTemplateLevelModelFromRecord(levelRecord)
		var actionRecords []CaseTemplateLevelActionRecord
		if err := db.Where("level_id = ?", level.ID).Order("position ASC").Find(&actionRecords).Error; err != nil {
			return nil, fmt.Errorf("get case template level actions: %w", err)
		}
		actions := make([]model.CaseTemplateLevelAction, 0, len(actionRecords))
		for _, actionRecord := range actionRecords {
			actions = append(actions, caseTemplateLevelActionModelFromRecord(actionRecord))
		}
		expandedLevels = append(expandedLevels, ExpandedCaseTemplateLevel{
			Level:   level,
			Actions: actions,
		})
	}

	return &ExpandedCaseTemplate{
		Template: template,
		Levels:   expandedLevels,
	}, nil
}

// createTemplateLevels creates template levels while preserving validation, authorization, and persistence invariants.
func createTemplateLevels(tx *gorm.DB, templateID string, levels []ExpandedCaseTemplateLevel, now time.Time) error {
	for i := range levels {
		level := levels[i].Level
		level.TemplateID = templateID
		if err := prepareULIDModel(&level.ULIDModel, now); err != nil {
			return fmt.Errorf("prepare case template level model: %w", err)
		}
		levelRecord := caseTemplateLevelRecordFromModel(level)
		if err := tx.Select("*").Create(&levelRecord).Error; err != nil {
			return fmt.Errorf("create case template level: %w", err)
		}

		for j := range levels[i].Actions {
			action := levels[i].Actions[j]
			action.LevelID = level.ID
			if err := prepareULIDModel(&action.ULIDModel, now); err != nil {
				return fmt.Errorf("prepare case template level action model: %w", err)
			}
			actionRecord := caseTemplateLevelActionRecordFromModel(action)
			if err := tx.Select("*").Create(&actionRecord).Error; err != nil {
				return fmt.Errorf("create case template level action: %w", err)
			}
		}
	}

	return nil
}

// caseTemplateRecordFromModel maps the live template model into the compatibility storage shape.
func caseTemplateRecordFromModel(template model.CaseTemplate) CaseTemplateRecord {
	return CaseTemplateRecord{
		ULIDModelRecord:        ulidRecordFromModel(template.ULIDModel),
		GuildID:                template.GuildID,
		Slug:                   template.Slug,
		Name:                   template.Name,
		Description:            template.Description,
		ReasonTemplate:         template.ReasonTemplate,
		DefaultSeverity:        model.CaseSeverityMedium,
		Appealable:             template.Appealable,
		Enabled:                true,
		Version:                template.Version,
		CreatedByDiscordUserID: template.CreatedByDiscordUserID,
		UpdatedByDiscordUserID: template.UpdatedByDiscordUserID,
		ArchivedAt:             template.ArchivedAt,
	}
}

// caseTemplateModelFromRecord omits retired compatibility columns from live behavior.
func caseTemplateModelFromRecord(record CaseTemplateRecord) model.CaseTemplate {
	return model.CaseTemplate{
		ULIDModel:              ulidModelFromRecord(record.ULIDModelRecord),
		GuildID:                record.GuildID,
		Slug:                   record.Slug,
		Name:                   record.Name,
		Description:            record.Description,
		ReasonTemplate:         record.ReasonTemplate,
		Appealable:             record.Appealable,
		Version:                record.Version,
		CreatedByDiscordUserID: record.CreatedByDiscordUserID,
		UpdatedByDiscordUserID: record.UpdatedByDiscordUserID,
		ArchivedAt:             record.ArchivedAt,
	}
}

// caseTemplateLevelRecordFromModel stores live level state with inert compatibility defaults.
func caseTemplateLevelRecordFromModel(level model.CaseTemplateLevel) CaseTemplateLevelRecord {
	return CaseTemplateLevelRecord{
		ULIDModelRecord:  ulidRecordFromModel(level.ULIDModel),
		TemplateID:       level.TemplateID,
		Position:         level.Position,
		Name:             level.Name,
		IsDefault:        level.IsDefault,
		TriggerCaseCount: level.TriggerCaseCount,
		NotifyUser:       level.NotifyUser,
		Enabled:          true,
	}
}

// caseTemplateLevelModelFromRecord omits retired compatibility columns from live behavior.
func caseTemplateLevelModelFromRecord(record CaseTemplateLevelRecord) model.CaseTemplateLevel {
	return model.CaseTemplateLevel{
		ULIDModel:        ulidModelFromRecord(record.ULIDModelRecord),
		TemplateID:       record.TemplateID,
		Position:         record.Position,
		Name:             record.Name,
		IsDefault:        record.IsDefault,
		TriggerCaseCount: record.TriggerCaseCount,
		NotifyUser:       record.NotifyUser,
	}
}

// caseTemplateLevelActionRecordFromModel stores one live action with inert compatibility defaults.
func caseTemplateLevelActionRecordFromModel(action model.CaseTemplateLevelAction) CaseTemplateLevelActionRecord {
	return CaseTemplateLevelActionRecord{
		ULIDModelRecord:  ulidRecordFromModel(action.ULIDModel),
		LevelID:          action.LevelID,
		Position:         1,
		ActionType:       action.ActionType,
		ConfigJSON:       action.ConfigJSON,
		MaxRetries:       action.MaxRetries,
		IdempotencyScope: "case",
		Enabled:          true,
	}
}

// caseTemplateLevelActionModelFromRecord omits retired compatibility columns from live behavior.
func caseTemplateLevelActionModelFromRecord(record CaseTemplateLevelActionRecord) model.CaseTemplateLevelAction {
	return model.CaseTemplateLevelAction{
		ULIDModel:  ulidModelFromRecord(record.ULIDModelRecord),
		LevelID:    record.LevelID,
		ActionType: record.ActionType,
		ConfigJSON: record.ConfigJSON,
		MaxRetries: record.MaxRetries,
	}
}

// ulidRecordFromModel maps shared identifier and timestamp fields into storage.
func ulidRecordFromModel(value model.ULIDModel) ULIDModelRecord {
	return ULIDModelRecord{ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

// ulidModelFromRecord maps shared identifier and timestamp fields into the domain.
func ulidModelFromRecord(value ULIDModelRecord) model.ULIDModel {
	return model.ULIDModel{ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

// createAuditLogEntry creates audit log entry while preserving validation, authorization, and persistence invariants.
func createAuditLogEntry(db *gorm.DB, entry *model.AuditLogEntry, now time.Time) error {
	if entry == nil {
		return nil
	}
	if entry.ResourceID == "" {
		entry.ResourceID = "unknown"
	}
	if entry.MetadataJSON == "" {
		entry.MetadataJSON = "{}"
	}
	if err := prepareULIDModel(&entry.ULIDModel, now); err != nil {
		return fmt.Errorf("prepare audit log entry model: %w", err)
	}
	if err := db.Create(entry).Error; err != nil {
		return fmt.Errorf("create audit log entry: %w", err)
	}

	return nil
}
