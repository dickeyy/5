package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const migration0002Definition = `simplify-template-model-v1
mode: preserve-and-quarantine
live model: archive is the only availability signal; levels have all-time distinct thresholds and zero or one timeout, kick, or ban action
compatibility: retain every legacy template, level, action, case snapshot, and audit row; archive active templates whose removed settings cannot be represented safely; convert legacy soft deletion into archive state
bookkeeping: quack_v5_0002_template_compatibility records prior archive/deletion state and quarantine reasons
rollback: restore recorded archive/deletion timestamps and remove only migration-owned bookkeeping`

const (
	migration0002MaxRetries           = 10
	migration0002MaxTimeoutSeconds    = 28 * 24 * 60 * 60
	migration0002MaxBanHistorySeconds = 7 * 24 * 60 * 60
)

// migration0002SimplifyTemplateModel quarantines incompatible live configurations without deleting or rewriting them.
func migration0002SimplifyTemplateModel() migration {
	return migration{
		Version:    2,
		Name:       "simplify_template_model",
		Definition: migration0002Definition,
		Source:     migration0002Source,
		Up:         applyTemplateModelCompatibility,
		Down:       rollbackTemplateModelCompatibility,
	}
}

// migration0002TemplateCompatibility records only archive changes owned by migration 0002.
type migration0002TemplateCompatibility struct {
	TemplateID         string `gorm:"type:char(26);primaryKey"`
	PreviousArchivedAt *time.Time
	PreviousDeletedAt  *time.Time
	Reason             string    `gorm:"type:text;not null"`
	RecordedAt         time.Time `gorm:"not null"`
}

// TableName gives migration-owned compatibility state a stable isolated name.
func (migration0002TemplateCompatibility) TableName() string {
	return "quack_v5_0002_template_compatibility"
}

// migration0002Template is the frozen subset needed to identify incompatible active templates.
type migration0002Template struct {
	ID              string
	DefaultSeverity string
	Enabled         bool
	ArchivedAt      *time.Time
	DeletedAt       gorm.DeletedAt
}

// TableName keeps migration reads on the existing v5 template table.
func (migration0002Template) TableName() string { return "case_templates" }

// migration0002Level is the frozen subset needed to validate one template's levels.
type migration0002Level struct {
	ID               string
	TemplateID       string
	IsDefault        bool
	TriggerCaseCount int
	WindowMinutes    int
	Enabled          bool
}

// TableName keeps migration reads on the existing v5 level table.
func (migration0002Level) TableName() string { return "case_template_levels" }

// migration0002Action is the frozen subset needed to validate one level's action.
type migration0002Action struct {
	LevelID          string
	ActionType       string
	ConfigJSON       string `gorm:"column:config_json"`
	NotifyUser       bool
	NotificationType string
	ContinueOnError  bool
	MaxRetries       uint8
	RetryBackoffMS   int
	TimeoutMS        int
	IdempotencyScope string
	Enabled          bool
}

// TableName keeps migration reads on the existing v5 action table.
func (migration0002Action) TableName() string { return "case_template_level_actions" }

// applyTemplateModelCompatibility archives active templates whose removed behavior cannot be honored safely.
func applyTemplateModelCompatibility(db *gorm.DB) error {
	migrator := withMySQLTableOptions(db).Migrator()
	if !migrator.HasTable(&migration0002TemplateCompatibility{}) {
		if err := migrator.CreateTable(&migration0002TemplateCompatibility{}); err != nil {
			return fmt.Errorf("create template compatibility table: %w", err)
		}
	}

	var templates []migration0002Template
	if err := db.Unscoped().Find(&templates).Error; err != nil {
		return fmt.Errorf("list templates for compatibility: %w", err)
	}
	for _, template := range templates {
		if template.ArchivedAt != nil && !template.DeletedAt.Valid {
			continue
		}
		reasons, err := migration0002TemplateReasons(db, template)
		if err != nil {
			return err
		}
		if len(reasons) == 0 {
			continue
		}
		sort.Strings(reasons)
		entry := migration0002TemplateCompatibility{
			TemplateID:         template.ID,
			PreviousArchivedAt: template.ArchivedAt,
			PreviousDeletedAt:  deletedAtPointer(template.DeletedAt),
			Reason:             strings.Join(reasons, "; "),
			RecordedAt:         time.Now().UTC(),
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
			return fmt.Errorf("record incompatible template %s: %w", template.ID, err)
		}
		archivedAt := template.ArchivedAt
		if archivedAt == nil {
			value := time.Now().UTC()
			archivedAt = &value
		}
		if err := db.Unscoped().Model(&migration0002Template{}).
			Where("id = ?", template.ID).
			Updates(map[string]any{"archived_at": archivedAt, "deleted_at": nil}).Error; err != nil {
			return fmt.Errorf("archive incompatible template %s: %w", template.ID, err)
		}
	}
	return nil
}

// migration0002TemplateReasons returns every compatibility reason without mutating source rows.
func migration0002TemplateReasons(db *gorm.DB, template migration0002Template) ([]string, error) {
	reasons := make([]string, 0)
	if template.DeletedAt.Valid {
		reasons = append(reasons, "template used legacy soft deletion")
	}
	if !template.Enabled {
		reasons = append(reasons, "template disabled")
	}
	if template.DefaultSeverity != "" && template.DefaultSeverity != "medium" {
		reasons = append(reasons, "template severity is not the compatibility default")
	}

	var levels []migration0002Level
	if err := db.Where("template_id = ?", template.ID).Find(&levels).Error; err != nil {
		return nil, fmt.Errorf("list levels for template %s: %w", template.ID, err)
	}
	defaultCount := 0
	thresholds := make(map[int]struct{}, len(levels))
	for _, level := range levels {
		if !level.Enabled {
			reasons = append(reasons, "level disabled")
		}
		if level.WindowMinutes != 0 {
			reasons = append(reasons, "level uses an escalation window")
		}
		if level.IsDefault {
			defaultCount++
			if level.TriggerCaseCount != 0 {
				reasons = append(reasons, "default level has a threshold")
			}
		} else if level.TriggerCaseCount <= 0 {
			reasons = append(reasons, "non-default level has a non-positive threshold")
		} else if _, exists := thresholds[level.TriggerCaseCount]; exists {
			reasons = append(reasons, "duplicate escalation threshold")
		} else {
			thresholds[level.TriggerCaseCount] = struct{}{}
		}

		var actions []migration0002Action
		if err := db.Where("level_id = ?", level.ID).Find(&actions).Error; err != nil {
			return nil, fmt.Errorf("list actions for level %s: %w", level.ID, err)
		}
		if len(actions) > 1 {
			reasons = append(reasons, "level has multiple actions")
		}
		for _, action := range actions {
			reasons = append(reasons, migration0002ActionReasons(action)...)
		}
	}
	if defaultCount != 1 {
		reasons = append(reasons, "template does not have exactly one default level")
	}
	return uniqueStrings(reasons), nil
}

// migration0002ActionReasons identifies settings whose behavior cannot survive the live model simplification.
func migration0002ActionReasons(action migration0002Action) []string {
	reasons := make([]string, 0)
	if action.ActionType != "timeout_user" && action.ActionType != "kick_user" && action.ActionType != "ban_user" {
		reasons = append(reasons, "unsupported configured action type")
	}
	if !action.Enabled {
		reasons = append(reasons, "action disabled")
	}
	if action.NotifyUser || strings.TrimSpace(action.NotificationType) != "" {
		reasons = append(reasons, "action-level notification configured")
	}
	if action.ContinueOnError {
		reasons = append(reasons, "continue-on-error configured")
	}
	if action.MaxRetries > migration0002MaxRetries {
		reasons = append(reasons, "safe retry count exceeds product limit")
	}
	if action.RetryBackoffMS != 0 || action.TimeoutMS != 0 || (action.IdempotencyScope != "" && action.IdempotencyScope != "case") {
		reasons = append(reasons, "admin-facing execution controls configured")
	}
	if !migration0002ValidActionConfig(action.ActionType, action.ConfigJSON) {
		reasons = append(reasons, "action settings are invalid for the simplified model")
	}
	return reasons
}

// migration0002ValidActionConfig accepts canonical settings and the one previous timeout representation that can be mapped losslessly.
func migration0002ValidActionConfig(actionType, body string) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &fields); err != nil || fields == nil {
		return false
	}
	readInt := func(name string) (int, bool) {
		raw, exists := fields[name]
		if !exists {
			return 0, false
		}
		var value int
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, false
		}
		return value, true
	}
	switch actionType {
	case "timeout_user":
		if len(fields) != 1 {
			return false
		}
		if seconds, ok := readInt("duration_seconds"); ok {
			return seconds > 0 && seconds <= migration0002MaxTimeoutSeconds
		}
		minutes, ok := readInt("duration_minutes")
		return ok && minutes > 0 && minutes <= migration0002MaxTimeoutSeconds/60
	case "kick_user":
		return len(fields) == 0
	case "ban_user":
		if len(fields) == 0 {
			return true
		}
		if len(fields) != 1 {
			return false
		}
		seconds, ok := readInt("delete_message_seconds")
		return ok && seconds >= 0 && seconds <= migration0002MaxBanHistorySeconds
	default:
		return false
	}
}

// rollbackTemplateModelCompatibility restores only archive state changed by migration 0002.
func rollbackTemplateModelCompatibility(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&migration0002TemplateCompatibility{}) {
		return nil
	}
	var entries []migration0002TemplateCompatibility
	if err := db.Find(&entries).Error; err != nil {
		return fmt.Errorf("list template compatibility records: %w", err)
	}
	for _, entry := range entries {
		if err := db.Unscoped().Model(&migration0002Template{}).
			Where("id = ?", entry.TemplateID).
			Updates(map[string]any{"archived_at": entry.PreviousArchivedAt, "deleted_at": entry.PreviousDeletedAt}).Error; err != nil {
			return fmt.Errorf("restore template %s archive state: %w", entry.TemplateID, err)
		}
	}
	if err := migrator.DropTable(&migration0002TemplateCompatibility{}); err != nil {
		return fmt.Errorf("drop template compatibility table: %w", err)
	}
	return nil
}

// deletedAtPointer returns the exact legacy deletion timestamp for reversible compatibility bookkeeping.
func deletedAtPointer(value gorm.DeletedAt) *time.Time {
	if !value.Valid {
		return nil
	}
	deletedAt := value.Time
	return &deletedAt
}

// uniqueStrings removes duplicate compatibility reasons while preserving deterministic content.
func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
