package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// migration0200AppealColumns adds nullable compatibility columns before preserved rows are backfilled.
type migration0200AppealColumns struct {
	ID                   string  `gorm:"type:char(26);primaryKey"`
	QuestionSnapshotJSON *string `gorm:"type:json"`
	AnswersJSON          *string `gorm:"type:json"`
	Version              *uint64 `gorm:"type:bigint unsigned"`
}

// TableName targets the preserved placeholder appeal table.
func (migration0200AppealColumns) TableName() string { return "appeals" }

// migration0200EventColumns adds nullable actor classification before preserved timelines are backfilled.
type migration0200EventColumns struct {
	ID        string  `gorm:"type:char(26);primaryKey"`
	ActorType *string `gorm:"size:32"`
}

// TableName targets the preserved placeholder appeal event table.
func (migration0200EventColumns) TableName() string { return "appeal_events" }

// migration0200LegacyAppeal is the minimum compatibility fixture used by migration logic.
type migration0200LegacyAppeal struct {
	ID                      string  `gorm:"type:char(26);primaryKey"`
	GuildID                 string  `gorm:"type:char(26);not null"`
	CaseID                  *string `gorm:"type:char(26)"`
	TargetDiscordUserID     string  `gorm:"size:32;not null"`
	Status                  string  `gorm:"size:32;not null"`
	Content                 string  `gorm:"type:text;not null"`
	DecisionReason          string  `gorm:"type:text"`
	ReviewedByDiscordUserID string  `gorm:"size:32"`
	ReviewedAt              *time.Time
	ReviewMessageDiscordID  string `gorm:"size:32"`
	MetadataJSON            string `gorm:"type:json;not null"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	QuestionSnapshotJSON    *string
	AnswersJSON             *string
	Version                 *uint64
}

// TableName targets preserved appeal rows.
func (migration0200LegacyAppeal) TableName() string { return "appeals" }

const migration0200Definition = `appeals-and-member-access-v1
logical migration: 0200 appeals_and_member_access
schema: case-unique appeals with question and answer snapshots, immutable actor-typed timeline, guild appeal form settings, and leased notification outbox
invariants: one non-null appeal per case; member/staff notification bodies are independent of staff identity; accepted decisions atomically void cases and cancel queued enforcement/notification work
rollback: forward-only because appeal timelines and decisions are permanent moderation history`

// migration0200Appeals returns the logical 0200 migration at an integration-assigned contiguous ledger version.
func migration0200Appeals(version uint64) migration {
	return migration{
		Version: version, Name: "appeals_and_member_access_0200",
		Definition: migration0200Definition, Source: migration0200Source,
		Up: applyAppealSchema,
	}
}

// applyAppealSchema upgrades placeholder appeal tables and creates package-owned settings and outbox tables.
func applyAppealSchema(db *gorm.DB) error {
	var duplicateCases int64
	if db.Migrator().HasTable(&appealV5Record{}) {
		row := db.Raw(`SELECT COUNT(*) FROM (SELECT case_id FROM appeals WHERE case_id IS NOT NULL GROUP BY case_id HAVING COUNT(*) > 1) appeal_duplicates`).Scan(&duplicateCases)
		if row.Error != nil {
			return fmt.Errorf("inspect duplicate case appeals: %w", row.Error)
		}
		if duplicateCases > 0 {
			return fmt.Errorf("appeal migration requires adjudication of %d duplicate case groups", duplicateCases)
		}
	}
	if err := withMySQLTableOptions(db).AutoMigrate(&migration0200AppealColumns{}, &migration0200EventColumns{}); err != nil {
		return err
	}
	if err := backfillAppealSnapshots(db); err != nil {
		return err
	}
	if err := db.Exec(`UPDATE appeal_events SET actor_type = CASE WHEN event_type IN ('submitted', 'information_submitted') THEN 'member' WHEN actor_discord_user_id <> '' THEN 'staff' ELSE 'system' END WHERE actor_type IS NULL OR actor_type = ''`).Error; err != nil {
		return err
	}
	if err := withMySQLTableOptions(db).AutoMigrate(
		&appealV5Record{}, &appealEventV5Record{},
		&GuildAppealSettingsRecord{}, &AppealNotificationRecord{},
	); err != nil {
		return err
	}
	return nil
}

// backfillAppealSnapshots makes preserved placeholder content readable through the final structured form contract.
func backfillAppealSnapshots(db *gorm.DB) error {
	var rows []migration0200LegacyAppeal
	if err := db.Find(&rows).Error; err != nil {
		return err
	}
	questionBody, err := json.Marshal([]map[string]any{{"id": "legacy_content", "prompt": "Appeal statement", "type": "long_text", "required": true, "position": 0}})
	if err != nil {
		return err
	}
	for _, row := range rows {
		updates := map[string]any{}
		if row.QuestionSnapshotJSON == nil || strings.TrimSpace(*row.QuestionSnapshotJSON) == "" {
			updates["question_snapshot_json"] = string(questionBody)
		}
		if row.AnswersJSON == nil || strings.TrimSpace(*row.AnswersJSON) == "" {
			answerBody, marshalErr := json.Marshal([]map[string]any{{"question_id": "legacy_content", "value": row.Content}})
			if marshalErr != nil {
				return marshalErr
			}
			updates["answers_json"] = string(answerBody)
		}
		if row.Version == nil || *row.Version == 0 {
			updates["version"] = 1
		}
		if len(updates) != 0 {
			if err := db.Model(&migration0200AppealColumns{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
