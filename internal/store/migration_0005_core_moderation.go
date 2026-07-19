package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

const migration0005Definition = `core-moderation-runtime-v1
schema: add ordered template context definitions, immutable case context/correction links, evidence snapshots and attachment copies, fenced action leases/reversals, and exactly-one case notification
compatibility: existing templates have no required context; existing cases receive empty context and correction fields; existing action rows remain claimable with empty leases; no moderation history is rewritten
rollback: remove migration-owned tables and additive columns only; pre-v5 compatibility tables and rows remain intact`

// migration0005CoreModeration adds the durable core runtime boundaries used by v5 cases.
func migration0005CoreModeration() migration {
	return migration{Version: 5, Name: "core_moderation_runtime", Definition: migration0005Definition, Source: migration0005Source, Up: applyCoreModeration, Down: rollbackCoreModeration}
}

type migration0005ContextField struct {
	ID         string    `gorm:"type:char(26);primaryKey"`
	CreatedAt  time.Time `gorm:"not null;index"`
	UpdatedAt  time.Time `gorm:"not null"`
	TemplateID string    `gorm:"type:char(26);not null;uniqueIndex:idx_template_context_key,priority:1;uniqueIndex:idx_template_context_position,priority:1;index"`
	Key        string    `gorm:"size:64;not null;uniqueIndex:idx_template_context_key,priority:2"`
	Label      string    `gorm:"size:191;not null"`
	FieldType  string    `gorm:"size:32;not null"`
	Position   int       `gorm:"not null;uniqueIndex:idx_template_context_position,priority:2"`
	Required   bool      `gorm:"not null;default:false"`
}

// TableName freezes the template context table name for migration 0005.
func (migration0005ContextField) TableName() string { return "case_template_context_fields" }

type migration0005Evidence struct {
	ID                                                      string    `gorm:"type:char(26);primaryKey"`
	CreatedAt                                               time.Time `gorm:"not null;index"`
	UpdatedAt                                               time.Time `gorm:"not null"`
	CaseID                                                  string    `gorm:"type:char(26);not null;index"`
	GuildID                                                 string    `gorm:"type:char(26);not null;index"`
	ChannelDiscordID, MessageDiscordID, AuthorDiscordUserID string    `gorm:"size:32;not null"`
	MessageURL                                              string    `gorm:"size:1024;not null"`
	Content                                                 string    `gorm:"type:text;not null"`
	MessageCreatedAt                                        time.Time `gorm:"not null"`
	MessageEditedAt                                         *time.Time
	EmbedsJSON                                              string `gorm:"type:json;not null"`
	CaptureOutcome                                          string `gorm:"size:32;not null"`
	CaptureWarning                                          string `gorm:"type:text;not null"`
}

// TableName freezes the evidence snapshot table name for migration 0005.
func (migration0005Evidence) TableName() string { return "case_evidence_snapshots" }

type migration0005Attachment struct {
	ID                                                      string    `gorm:"type:char(26);primaryKey"`
	CreatedAt                                               time.Time `gorm:"not null;index"`
	UpdatedAt                                               time.Time `gorm:"not null"`
	EvidenceID                                              string    `gorm:"type:char(26);not null;index"`
	Filename                                                string    `gorm:"size:255;not null"`
	ContentType                                             string    `gorm:"size:191;not null"`
	SizeBytes                                               int64     `gorm:"not null"`
	OriginalURL, PreservedURL                               string    `gorm:"size:2048;not null"`
	PreservedMessageDiscordID, PreservedAttachmentDiscordID string    `gorm:"size:32;not null"`
	CopyOutcome                                             string    `gorm:"size:32;not null"`
	Warning                                                 string    `gorm:"type:text;not null"`
}

// TableName freezes the evidence attachment table name for migration 0005.
func (migration0005Attachment) TableName() string { return "case_evidence_attachments" }

type migration0005Notification struct {
	ID                       string     `gorm:"type:char(26);primaryKey"`
	CreatedAt                time.Time  `gorm:"not null;index"`
	UpdatedAt                time.Time  `gorm:"not null"`
	CaseID                   string     `gorm:"type:char(26);not null;uniqueIndex"`
	Status                   string     `gorm:"size:32;not null;index"`
	PreparedChannelDiscordID string     `gorm:"size:32;not null"`
	RenderedMessage          string     `gorm:"type:text;not null"`
	DeliveryMessageDiscordID string     `gorm:"size:32;not null"`
	AttemptCount             uint8      `gorm:"not null;default:0"`
	LastErrorCode            string     `gorm:"size:64;not null"`
	LastError                string     `gorm:"type:text;not null"`
	LeaseToken               string     `gorm:"size:64;index"`
	LeaseExpiresAt           *time.Time `gorm:"index"`
	SentAt                   *time.Time
}

// TableName freezes the case notification table name for migration 0005.
func (migration0005Notification) TableName() string { return "case_notifications" }

type migration0005CaseColumns struct {
	GuildID               string     `gorm:"type:char(26);not null;uniqueIndex:idx_case_guild_idempotency,priority:1"`
	ContextValuesJSON     string     `gorm:"type:json"`
	VoidedReason          string     `gorm:"type:text"`
	VoidedByDiscordUserID string     `gorm:"size:32;not null;default:''"`
	VoidedAt              *time.Time `gorm:"index"`
	ReplacementCaseID     *string    `gorm:"type:char(26);index"`
	ReplacesCaseID        *string    `gorm:"type:char(26);index"`
	IdempotencyKey        *string    `gorm:"size:191;uniqueIndex:idx_case_guild_idempotency,priority:2"`
}

// TableName targets the existing cases table for additive migration columns.
func (migration0005CaseColumns) TableName() string { return "cases" }

type migration0005ActionColumns struct {
	LeaseToken               string     `gorm:"size:64;index"`
	LeaseExpiresAt           *time.Time `gorm:"index"`
	DismissedAt              *time.Time `gorm:"index"`
	DismissedByDiscordUserID string     `gorm:"size:32;not null;default:''"`
	ReversalOfExecutionID    *string    `gorm:"type:char(26);index"`
	ReversalAppealID         *string    `gorm:"type:char(26);index"`
}

// TableName targets the existing action execution table for leases and reversals.
func (migration0005ActionColumns) TableName() string { return "case_action_executions" }

// applyCoreModeration creates additive durable runtime state without rewriting history.
func applyCoreModeration(db *gorm.DB) error {
	m := withMySQLTableOptions(db).Migrator()
	for _, table := range []any{&migration0005ContextField{}, &migration0005Evidence{}, &migration0005Attachment{}, &migration0005Notification{}} {
		if !m.HasTable(table) {
			if err := m.CreateTable(table); err != nil {
				return fmt.Errorf("create core moderation table %T: %w", table, err)
			}
		}
	}
	for _, column := range []string{"ContextValuesJSON", "VoidedReason", "VoidedByDiscordUserID", "VoidedAt", "ReplacementCaseID", "ReplacesCaseID", "IdempotencyKey"} {
		if !m.HasColumn(&migration0005CaseColumns{}, column) {
			if err := m.AddColumn(&migration0005CaseColumns{}, column); err != nil {
				return fmt.Errorf("add cases.%s: %w", column, err)
			}
		}
	}
	for _, column := range []string{"LeaseToken", "LeaseExpiresAt", "DismissedAt", "DismissedByDiscordUserID", "ReversalOfExecutionID", "ReversalAppealID"} {
		if !m.HasColumn(&migration0005ActionColumns{}, column) {
			if err := m.AddColumn(&migration0005ActionColumns{}, column); err != nil {
				return fmt.Errorf("add action executions.%s: %w", column, err)
			}
		}
	}
	for _, index := range []struct {
		model any
		field string
	}{{&migration0005CaseColumns{}, "VoidedAt"}, {&migration0005CaseColumns{}, "ReplacementCaseID"}, {&migration0005CaseColumns{}, "ReplacesCaseID"}, {&migration0005CaseColumns{}, "IdempotencyKey"}, {&migration0005ActionColumns{}, "LeaseToken"}, {&migration0005ActionColumns{}, "LeaseExpiresAt"}, {&migration0005ActionColumns{}, "DismissedAt"}, {&migration0005ActionColumns{}, "ReversalOfExecutionID"}, {&migration0005ActionColumns{}, "ReversalAppealID"}} {
		if !m.HasIndex(index.model, index.field) {
			if err := m.CreateIndex(index.model, index.field); err != nil {
				return fmt.Errorf("create core moderation index %s: %w", index.field, err)
			}
		}
	}
	return db.Model(&migration0005CaseColumns{}).Where("context_values_json IS NULL OR context_values_json = ''").Update("context_values_json", "[]").Error
}

// rollbackCoreModeration removes only migration-owned tables and columns.
func rollbackCoreModeration(db *gorm.DB) error {
	m := db.Migrator()
	for _, table := range []any{&migration0005Notification{}, &migration0005Attachment{}, &migration0005Evidence{}, &migration0005ContextField{}} {
		if m.HasTable(table) {
			if err := m.DropTable(table); err != nil {
				return err
			}
		}
	}
	for _, column := range []string{"IdempotencyKey", "ReplacesCaseID", "ReplacementCaseID", "VoidedAt", "VoidedByDiscordUserID", "VoidedReason", "ContextValuesJSON"} {
		if m.HasColumn(&migration0005CaseColumns{}, column) {
			if err := m.DropColumn(&migration0005CaseColumns{}, column); err != nil {
				return err
			}
		}
	}
	for _, column := range []string{"ReversalAppealID", "ReversalOfExecutionID", "DismissedByDiscordUserID", "DismissedAt", "LeaseExpiresAt", "LeaseToken"} {
		if m.HasColumn(&migration0005ActionColumns{}, column) {
			if err := m.DropColumn(&migration0005ActionColumns{}, column); err != nil {
				return err
			}
		}
	}
	return nil
}
