package honeypot

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

// LegacySettings is one explicit v4 honeypot settings source record.
type LegacySettings struct {
	SourceID, GuildID string
	Enabled           bool
	Settings          Settings
}

// ImportResult reports whether a v4 settings row would be or was mapped.
type ImportResult struct {
	SourceID             string
	WouldCreate, Created bool
}

// Importer performs a module-only, validated v4 honeypot settings migration.
type Importer struct {
	db        *gorm.DB
	auditor   modules.Auditor
	channels  ChannelValidator
	templates TemplateValidator
	now       func() time.Time
}

// NewImporter constructs the honeypot importer without access to core history.
func NewImporter(db *gorm.DB, auditor modules.Auditor, channels ChannelValidator, templates TemplateValidator) *Importer {
	return &Importer{db: db, auditor: auditor, channels: channels, templates: templates, now: func() time.Time { return time.Now().UTC() }}
}

// Import validates all rows, supports side-effect-free dry runs, and uses the shared idempotency ledger.
func (i *Importer) Import(ctx context.Context, actor Actor, rows []LegacySettings, dryRun bool) ([]ImportResult, error) {
	if !actor.CanManage {
		return nil, ErrPermissionDenied
	}
	if i == nil || i.db == nil {
		return nil, errors.New("honeypot importer database is not connected")
	}
	results := make([]ImportResult, 0, len(rows))
	for _, row := range rows {
		row.Settings = normalizeSettings(row.Settings)
		row.Settings.DisabledReason = ""
		if row.SourceID == "" || row.GuildID != actor.GuildID {
			return nil, errors.New("invalid v4 honeypot settings row")
		}
		if err := validateSettings(row.Settings, row.Enabled); err != nil {
			return nil, err
		}
		if row.Enabled {
			if i.channels == nil || i.templates == nil {
				return nil, errors.New("honeypot import validators are not configured")
			}
			if err := i.channels.ValidateHoneypotChannel(ctx, row.GuildID, row.Settings.ChannelDiscordID); err != nil {
				return nil, fmtUnavailable(ErrChannelUnavailable, err)
			}
			if err := i.templates.ValidateHoneypotTemplate(ctx, row.GuildID, row.Settings.TemplateID); err != nil {
				return nil, fmtUnavailable(ErrTemplateUnavailable, err)
			}
		}
		var prior modules.ImportRecord
		query := i.db.WithContext(ctx).Where("guild_id = ? AND module_id = ? AND source_id = ?", row.GuildID, modules.Honeypots, row.SourceID).Limit(1).Find(&prior)
		if query.Error != nil {
			return nil, query.Error
		}
		if dryRun {
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
		_ = i.auditor.RecordModuleAudit(ctx, modules.AuditEvent{GuildID: actor.GuildID, ActorDiscordUserID: actor.DiscordUserID, Action: "honeypot.v4_settings_import", ResourceType: "honeypot_settings_import", Result: "success", MetadataJSON: "{}"})
	}
	return results, nil
}

func (i *Importer) importOne(ctx context.Context, row LegacySettings) (bool, error) {
	created := false
	err := i.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prior modules.ImportRecord
		query := tx.Where("guild_id = ? AND module_id = ? AND source_id = ?", row.GuildID, modules.Honeypots, row.SourceID).Limit(1).Find(&prior)
		if query.Error != nil || query.RowsAffected > 0 {
			return query.Error
		}
		payload, err := json.Marshal(row.Settings)
		if err != nil {
			return err
		}
		now := i.now()
		configuration := modules.Configuration{ID: ulid.Make().String(), GuildID: row.GuildID, ModuleID: modules.Honeypots, Enabled: row.Enabled, ConfigJSON: string(payload), CreatedAt: now, UpdatedAt: now}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "guild_id"}, {Name: "module_id"}}, DoUpdates: clause.AssignmentColumns([]string{"enabled", "config_json", "updated_at"})}).Create(&configuration).Error; err != nil {
			return err
		}
		if err := tx.Where("guild_id = ? AND module_id = ?", row.GuildID, modules.Honeypots).First(&configuration).Error; err != nil {
			return err
		}
		ledger := modules.ImportRecord{ID: ulid.Make().String(), GuildID: row.GuildID, ModuleID: modules.Honeypots, SourceID: row.SourceID, TargetID: configuration.ID, CreatedAt: now}
		if err := tx.Create(&ledger).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	return created, err
}

func fmtUnavailable(kind, cause error) error {
	return errors.Join(kind, cause)
}
