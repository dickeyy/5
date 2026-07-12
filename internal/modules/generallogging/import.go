package generallogging

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/quackdiscord/bot/internal/modules"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LegacySettings is one explicit v4 guild logging-settings source record.
type LegacySettings struct {
	SourceID, GuildID string
	Enabled           bool
	Settings          Settings
}

// ImportResult reports whether a settings row would be or was created.
type ImportResult struct {
	SourceID             string
	WouldCreate, Created bool
}

// Importer performs logging-settings migration without importing old event content.
type Importer struct {
	db      *gorm.DB
	auditor modules.Auditor
	now     func() time.Time
}

// NewImporter constructs the module-only v4 settings importer.
func NewImporter(db *gorm.DB, auditor modules.Auditor) *Importer {
	return &Importer{db: db, auditor: auditor, now: func() time.Time { return time.Now().UTC() }}
}

// Import validates, dry-runs, and idempotently maps settings while never importing general-log events.
func (i *Importer) Import(ctx context.Context, actor Actor, rows []LegacySettings, dryRun bool) ([]ImportResult, error) {
	if !actor.CanManage {
		return nil, ErrPermissionDenied
	}
	results := make([]ImportResult, 0, len(rows))
	for _, row := range rows {
		if row.SourceID == "" || row.GuildID != actor.GuildID {
			return nil, errors.New("invalid v4 logging settings row")
		}
		if err := validateSettings(row.Settings, row.Enabled); err != nil {
			return nil, err
		}
		if dryRun {
			var prior modules.ImportRecord
			query := i.db.WithContext(ctx).Where("guild_id = ? AND module_id = ? AND source_id = ?", row.GuildID, modules.GeneralLogging, row.SourceID).Limit(1).Find(&prior)
			if query.Error != nil {
				return nil, query.Error
			}
			results = append(results, ImportResult{SourceID: row.SourceID, WouldCreate: query.RowsAffected == 0})
			continue
		}
		created, err := i.importOne(ctx, row)
		if err != nil {
			return nil, err
		}
		results = append(results, ImportResult{SourceID: row.SourceID, Created: created})
	}
	if !dryRun && i.auditor != nil {
		_ = i.auditor.RecordModuleAudit(ctx, modules.AuditEvent{GuildID: actor.GuildID, ActorDiscordUserID: actor.DiscordUserID, Action: "general_logging.v4_settings_import", ResourceType: "general_logging_settings_import", Result: "success", MetadataJSON: "{}"})
	}
	return results, nil
}
func (i *Importer) importOne(ctx context.Context, row LegacySettings) (bool, error) {
	created := false
	err := i.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prior modules.ImportRecord
		q := tx.Where("guild_id = ? AND module_id = ? AND source_id = ?", row.GuildID, modules.GeneralLogging, row.SourceID).Limit(1).Find(&prior)
		if q.Error != nil {
			return q.Error
		}
		if q.RowsAffected > 0 {
			return nil
		}
		payload, _ := json.Marshal(row.Settings)
		now := i.now()
		configuration := modules.Configuration{ID: ulid.Make().String(), GuildID: row.GuildID, ModuleID: modules.GeneralLogging, Enabled: row.Enabled, ConfigJSON: string(payload), CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "guild_id"}, {Name: "module_id"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "config_json", "updated_at"})}).Create(&configuration).Error; err != nil {
			return err
		}
		configuration = modules.Configuration{}
		if err := tx.Where("guild_id = ? AND module_id = ?", row.GuildID, modules.GeneralLogging).First(&configuration).Error; err != nil {
			return err
		}
		ledger := modules.ImportRecord{ID: ulid.Make().String(), GuildID: row.GuildID, ModuleID: modules.GeneralLogging, SourceID: row.SourceID, TargetID: configuration.ID, CreatedAt: now}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}
