package store

import (
	"time"

	"gorm.io/gorm"
)

const migration0008Definition = `honeypot-triggers-v1
logical migration: 0300 honeypot_triggers
schema: one guild-scoped deduplicated Discord message claim with selected template, resulting case, bounded outcome, and failure code
boundary: no message content, action payload, member history, or cross-module state
rollback: forward-only because trigger outcomes and case links are operator evidence`

// migration0008HoneypotTriggers reconciles logical module migration 0300 into
// the next contiguous position in the production ledger.
func migration0008HoneypotTriggers() migration {
	return migration{
		Version: 8, Name: "honeypot_triggers_0300",
		Definition: migration0008Definition, Source: migration0008Source,
		Up: applyHoneypotTriggers,
	}
}

// migration0008HoneypotTrigger freezes the isolated trigger ledger schema.
type migration0008HoneypotTrigger struct {
	ID                   string    `gorm:"type:char(26);primaryKey"`
	GuildID              string    `gorm:"type:char(26);not null;uniqueIndex:idx_honeypot_trigger,priority:1;index"`
	ChannelDiscordID     string    `gorm:"size:32;not null"`
	MessageDiscordID     string    `gorm:"size:32;not null;uniqueIndex:idx_honeypot_trigger,priority:2"`
	TargetDiscordUserID  string    `gorm:"size:32;not null"`
	TemplateID           string    `gorm:"type:char(26);not null"`
	CaseID               string    `gorm:"type:char(26)"`
	Outcome              string    `gorm:"size:32;not null"`
	FailureCode          string    `gorm:"size:64"`
	CreatedAt, UpdatedAt time.Time `gorm:"not null"`
}

// TableName identifies the isolated honeypot trigger ledger.
func (migration0008HoneypotTrigger) TableName() string { return "honeypot_triggers" }

// applyHoneypotTriggers creates logical migration 0300 from its frozen shape.
func applyHoneypotTriggers(db *gorm.DB) error {
	return withMySQLTableOptions(db).AutoMigrate(&migration0008HoneypotTrigger{})
}
