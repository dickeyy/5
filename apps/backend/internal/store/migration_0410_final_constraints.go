package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ActionManualReviewRecord marks an unsafe stale action for explicit operator adjudication without mutating execution history.
type ActionManualReviewRecord struct {
	ID          string    `gorm:"type:char(26);primaryKey"`
	ExecutionID string    `gorm:"type:char(26);not null;uniqueIndex"`
	CaseID      string    `gorm:"type:char(26);not null;index"`
	Reason      string    `gorm:"size:64;not null;index"`
	CreatedAt   time.Time `gorm:"not null"`
}

// TableName identifies the operator adjudication queue for unsafe action recovery.
func (ActionManualReviewRecord) TableName() string { return "action_manual_reviews" }

const migration0410Definition = `final-storage-constraints-v1
logical migration: 0410 final_storage_constraints
constraints: one default level and at most one enforcement action per level
indexes: member case history, case evidence lookup, audit cursor filters, and action claim recovery
recovery: expired running actions are copied to action_manual_reviews without modifying preserved executions
rollback: forward-only because removing final invariants could admit invalid production state`

// migration0410FinalStorageConstraints returns the logical 0410 migration at an integration-assigned contiguous ledger version.
func migration0410FinalStorageConstraints(version uint64) migration {
	return migration{Version: version, Name: "final_storage_constraints_0410", Definition: migration0410Definition, Source: migration0410Source, Up: applyFinalStorageConstraints}
}

func applyFinalStorageConstraints(db *gorm.DB) error {
	// A frozen legacy column remains for compatibility, but archive is the only live availability lifecycle.
	if err := db.Exec(`UPDATE case_templates SET archived_at = COALESCE(archived_at, deleted_at), deleted_at = NULL WHERE deleted_at IS NOT NULL`).Error; err != nil {
		return fmt.Errorf("retire legacy template soft-delete state: %w", err)
	}
	checks := []struct{ query, label string }{
		{`SELECT COUNT(*) FROM (SELECT template_id FROM case_template_levels WHERE is_default = 1 GROUP BY template_id HAVING COUNT(*) > 1) duplicates`, "templates with multiple default levels"},
		{`SELECT COUNT(*) FROM (SELECT level_id FROM case_template_level_actions GROUP BY level_id HAVING COUNT(*) > 1) duplicates`, "levels with multiple enforcement actions"},
	}
	for _, check := range checks {
		var count int64
		if err := db.Raw(check.query).Scan(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("final storage migration requires manual review: %d %s", count, check.label)
		}
	}
	if err := withMySQLTableOptions(db).AutoMigrate(&ActionManualReviewRecord{}); err != nil {
		return err
	}
	if db.Dialector.Name() == "mysql" {
		if !db.Migrator().HasColumn("case_template_levels", "default_template_id") {
			if err := db.Exec(`ALTER TABLE case_template_levels ADD COLUMN default_template_id CHAR(26) GENERATED ALWAYS AS (CASE WHEN is_default = 1 THEN template_id ELSE NULL END) STORED`).Error; err != nil {
				return err
			}
		}
		statements := []string{
			`CREATE UNIQUE INDEX uq_v5_template_default_level ON case_template_levels (default_template_id)`,
			`CREATE UNIQUE INDEX uq_v5_level_enforcement_action ON case_template_level_actions (level_id)`,
			`CREATE INDEX idx_v5_case_member_history ON cases (guild_id, target_discord_user_id, created_at, id)`,
			`CREATE INDEX idx_v5_case_evidence_lookup ON case_evidence_snapshots (case_id, message_discord_id)`,
			`CREATE INDEX idx_v5_audit_cursor ON audit_log_entries (guild_id, created_at, id)`,
			`CREATE INDEX idx_v5_action_claim ON case_action_executions (status, next_retry_at, lease_expires_at)`,
		}
		for _, statement := range statements {
			if err := createMySQLIndexUnlessPresent(db, statement); err != nil {
				return err
			}
		}
		if err := db.Exec(`INSERT IGNORE INTO action_manual_reviews (id, execution_id, case_id, reason, created_at) SELECT LEFT(SHA2(CONCAT('expired:', id), 256), 26), id, case_id, 'expired_running_action', UTC_TIMESTAMP(6) FROM case_action_executions WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < UTC_TIMESTAMP(6)`).Error; err != nil {
			return err
		}
		return nil
	}
	statements := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_v5_template_default_level ON case_template_levels (template_id) WHERE is_default = 1`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_v5_level_enforcement_action ON case_template_level_actions (level_id)`,
		`CREATE INDEX IF NOT EXISTS idx_v5_case_member_history ON cases (guild_id, target_discord_user_id, created_at, id)`,
		`CREATE INDEX IF NOT EXISTS idx_v5_case_evidence_lookup ON case_evidence_snapshots (case_id, message_discord_id)`,
		`CREATE INDEX IF NOT EXISTS idx_v5_audit_cursor ON audit_log_entries (guild_id, created_at, id)`,
		`CREATE INDEX IF NOT EXISTS idx_v5_action_claim ON case_action_executions (status, next_retry_at, lease_expires_at)`,
		`INSERT OR IGNORE INTO action_manual_reviews (id, execution_id, case_id, reason, created_at) SELECT substr('expired-' || id, 1, 26), id, case_id, 'expired_running_action', CURRENT_TIMESTAMP FROM case_action_executions WHERE status = 'running' AND lease_expires_at IS NOT NULL AND lease_expires_at < CURRENT_TIMESTAMP`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func createMySQLIndexUnlessPresent(db *gorm.DB, statement string) error {
	var name string
	if _, err := fmt.Sscanf(statement, "CREATE UNIQUE INDEX %s", &name); err != nil {
		_, _ = fmt.Sscanf(statement, "CREATE INDEX %s", &name)
	}
	name = trimIndexToken(name)
	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND index_name = ?`, name).Scan(&count).Error; err != nil {
		return err
	}
	if count != 0 {
		return nil
	}
	return db.Exec(statement).Error
}

func trimIndexToken(value string) string {
	for len(value) > 0 && (value[len(value)-1] == ' ' || value[len(value)-1] == '(') {
		value = value[:len(value)-1]
	}
	return value
}
