package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const migration0003Definition = `case-validity-v1
mode: lossless-known-value-mapping
live model: case validity is valid or voided; action and appeal progress remain separate; sources are dashboard, discord, honeypot, and v4_import
mapping: open/action_running/completed/failed/appealed -> valid; voided -> voided; api -> dashboard; discord_command -> discord; automation -> honeypot; import -> v4_import
validation: reject unknown status or source values before mutating any case
legacy events: preserve note_added/note_edited/note_deleted/status_changed byte-for-byte; inventory counts per affected case; exclude them from live v5 reads
bookkeeping: quack_v5_0003_case_compatibility stores exact prior status/source plus retired event counts
rollback: restore exact prior status/source values; map post-migration canonical cases to compatible legacy values; remove only migration-owned bookkeeping`

// migration0003CaseValidity converts known legacy case values without rewriting immutable history.
func migration0003CaseValidity() migration {
	return migration{
		Version:    3,
		Name:       "case_validity",
		Definition: migration0003Definition,
		Source:     migration0003Source,
		Up:         applyCaseValidityCompatibility,
		Down:       rollbackCaseValidityCompatibility,
	}
}

// migration0003CaseCompatibility stores the exact values needed for reversible case mapping and retired-event inventory.
type migration0003CaseCompatibility struct {
	CaseID           string    `gorm:"type:char(26);primaryKey"`
	PreviousStatus   string    `gorm:"size:32;not null"`
	PreviousSource   string    `gorm:"size:32;not null"`
	NoteEventCount   int64     `gorm:"not null;default:0"`
	StatusEventCount int64     `gorm:"not null;default:0"`
	RecordedAt       time.Time `gorm:"not null"`
}

// TableName isolates migration-owned compatibility bookkeeping from product tables.
func (migration0003CaseCompatibility) TableName() string {
	return "quack_v5_0003_case_compatibility"
}

// migration0003Case is the frozen subset of case rows transformed by migration 0003.
type migration0003Case struct {
	ID     string
	Status string
	Source string
}

// TableName keeps migration reads and writes on the existing cases table.
func (migration0003Case) TableName() string { return "cases" }

// migration0003EventCount is a frozen aggregate of retired event rows for one case.
type migration0003EventCount struct {
	CaseID string
	Count  int64
}

// applyCaseValidityCompatibility validates every row before recording and applying lossless mappings.
func applyCaseValidityCompatibility(db *gorm.DB) error {
	var cases []migration0003Case
	if err := db.Find(&cases).Error; err != nil {
		return fmt.Errorf("list cases for validity compatibility: %w", err)
	}

	type mappedCase struct {
		legacy migration0003Case
		status string
		source string
	}
	mapped := make([]mappedCase, 0, len(cases))
	for _, legacy := range cases {
		status, ok := migration0003Status(legacy.Status)
		if !ok {
			return fmt.Errorf("case %s has unknown legacy status %q", legacy.ID, legacy.Status)
		}
		source, ok := migration0003SourceValue(legacy.Source)
		if !ok {
			return fmt.Errorf("case %s has unknown legacy source %q", legacy.ID, legacy.Source)
		}
		mapped = append(mapped, mappedCase{legacy: legacy, status: status, source: source})
	}

	migrator := withMySQLTableOptions(db).Migrator()
	if !migrator.HasTable(&migration0003CaseCompatibility{}) {
		if err := migrator.CreateTable(&migration0003CaseCompatibility{}); err != nil {
			return fmt.Errorf("create case compatibility table: %w", err)
		}
	}

	noteCounts, err := migration0003EventCounts(db, []string{"note_added", "note_edited", "note_deleted"})
	if err != nil {
		return err
	}
	statusCounts, err := migration0003EventCounts(db, []string{"status_changed"})
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, item := range mapped {
		entry := migration0003CaseCompatibility{
			CaseID: item.legacy.ID, PreviousStatus: item.legacy.Status, PreviousSource: item.legacy.Source,
			NoteEventCount: noteCounts[item.legacy.ID], StatusEventCount: statusCounts[item.legacy.ID], RecordedAt: now,
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&entry).Error; err != nil {
			return fmt.Errorf("record case %s compatibility: %w", item.legacy.ID, err)
		}
		if err := db.Model(&migration0003Case{}).Where("id = ?", item.legacy.ID).
			Updates(map[string]any{"status": item.status, "source": item.source}).Error; err != nil {
			return fmt.Errorf("map case %s validity and source: %w", item.legacy.ID, err)
		}
	}
	return nil
}

// migration0003EventCounts inventories retired event rows without modifying them.
func migration0003EventCounts(db *gorm.DB, eventTypes []string) (map[string]int64, error) {
	var rows []migration0003EventCount
	if err := db.Table("case_events").Select("case_id, COUNT(*) AS count").
		Where("event_type IN ?", eventTypes).Group("case_id").Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("inventory retired case events: %w", err)
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.CaseID] = row.Count
	}
	return counts, nil
}

// migration0003Status maps every supported pre-v5 validity/status value explicitly.
func migration0003Status(value string) (string, bool) {
	switch value {
	case "open", "action_running", "completed", "failed", "appealed", "valid":
		return "valid", true
	case "voided":
		return "voided", true
	default:
		return "", false
	}
}

// migration0003SourceValue maps every supported pre-v5 and canonical case source explicitly.
func migration0003SourceValue(value string) (string, bool) {
	switch value {
	case "api", "dashboard":
		return "dashboard", true
	case "discord_command", "discord":
		return "discord", true
	case "automation", "honeypot":
		return "honeypot", true
	case "import", "v4_import":
		return "v4_import", true
	default:
		return "", false
	}
}

// rollbackCaseValidityCompatibility restores exact legacy values, downgrades new canonical rows, and removes only migration-owned bookkeeping.
func rollbackCaseValidityCompatibility(db *gorm.DB) error {
	migrator := db.Migrator()
	if !migrator.HasTable(&migration0003CaseCompatibility{}) {
		return nil
	}
	var postMigrationCases []migration0003Case
	if err := db.Raw(`SELECT candidate.id, candidate.status, candidate.source
		FROM cases AS candidate
		LEFT JOIN quack_v5_0003_case_compatibility AS compatibility ON compatibility.case_id = candidate.id
		WHERE compatibility.case_id IS NULL`).Scan(&postMigrationCases).Error; err != nil {
		return fmt.Errorf("list post-migration cases before case validity rollback: %w", err)
	}
	type legacyValues struct{ status, source string }
	postMigrationLegacy := make(map[string]legacyValues, len(postMigrationCases))
	for _, candidate := range postMigrationCases {
		status, ok := migration0003LegacyStatus(candidate.Status)
		if !ok {
			return fmt.Errorf("post-migration case %s has unknown canonical validity %q", candidate.ID, candidate.Status)
		}
		source, ok := migration0003LegacySource(candidate.Source)
		if !ok {
			return fmt.Errorf("post-migration case %s has unknown canonical source %q", candidate.ID, candidate.Source)
		}
		postMigrationLegacy[candidate.ID] = legacyValues{status: status, source: source}
	}
	var entries []migration0003CaseCompatibility
	if err := db.Find(&entries).Error; err != nil {
		return fmt.Errorf("list case compatibility records: %w", err)
	}
	for _, entry := range entries {
		if err := db.Model(&migration0003Case{}).Where("id = ?", entry.CaseID).
			Updates(map[string]any{"status": entry.PreviousStatus, "source": entry.PreviousSource}).Error; err != nil {
			return fmt.Errorf("restore case %s status and source: %w", entry.CaseID, err)
		}
	}
	for caseID, legacy := range postMigrationLegacy {
		if err := db.Model(&migration0003Case{}).Where("id = ?", caseID).
			Updates(map[string]any{"status": legacy.status, "source": legacy.source}).Error; err != nil {
			return fmt.Errorf("downgrade post-migration case %s status and source: %w", caseID, err)
		}
	}
	if err := migrator.DropTable(&migration0003CaseCompatibility{}); err != nil {
		return fmt.Errorf("drop case compatibility table: %w", err)
	}
	return nil
}

// migration0003LegacyStatus maps canonical validity to the compatible pre-0003 case state.
func migration0003LegacyStatus(value string) (string, bool) {
	switch value {
	case "valid":
		return "open", true
	case "voided":
		return "voided", true
	default:
		return "", false
	}
}

// migration0003LegacySource maps canonical sources to their reviewed pre-0003 equivalents.
func migration0003LegacySource(value string) (string, bool) {
	switch value {
	case "dashboard":
		return "api", true
	case "discord":
		return "discord_command", true
	case "honeypot":
		return "automation", true
	case "v4_import":
		return "import", true
	default:
		return "", false
	}
}
