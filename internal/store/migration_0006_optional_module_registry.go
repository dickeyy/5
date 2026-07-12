package store

import (
	"time"

	"gorm.io/gorm"
)

const migration0006Definition = `optional-module-registry-v1
logical migration: 0100 optional_module_registry
schema: create guild-scoped module_configurations and idempotent module_import_records
boundary: opaque settings and imports remain outside moderation cases, actions, appeals, and audit payload delivery
rollback: forward-only because module configuration and import identities are operator data`

// migration0006OptionalModuleRegistry reconciles logical module migration 0100
// into the central contiguous production ledger.
func migration0006OptionalModuleRegistry() migration {
	return migration{
		Version: 6, Name: "optional_module_registry_0100",
		Definition: migration0006Definition, Source: migration0006Source,
		Up: applyOptionalModuleRegistry,
	}
}

// migration0006Configuration freezes the logical 0100 settings envelope.
type migration0006Configuration struct {
	ID         string    `gorm:"type:char(26);primaryKey"`
	GuildID    string    `gorm:"type:char(26);not null;uniqueIndex:idx_module_configuration,priority:1"`
	ModuleID   string    `gorm:"size:64;not null;uniqueIndex:idx_module_configuration,priority:2"`
	Enabled    bool      `gorm:"not null;default:false"`
	ConfigJSON string    `gorm:"type:json;not null"`
	CreatedAt  time.Time `gorm:"not null"`
	UpdatedAt  time.Time `gorm:"not null"`
}

// TableName preserves the shared optional-module settings table.
func (migration0006Configuration) TableName() string { return "module_configurations" }

// migration0006ImportRecord freezes the logical 0100 idempotency ledger.
type migration0006ImportRecord struct {
	ID        string    `gorm:"type:char(26);primaryKey"`
	GuildID   string    `gorm:"type:char(26);not null;uniqueIndex:idx_module_import,priority:1"`
	ModuleID  string    `gorm:"size:64;not null;uniqueIndex:idx_module_import,priority:2"`
	SourceID  string    `gorm:"size:191;not null;uniqueIndex:idx_module_import,priority:3"`
	TargetID  string    `gorm:"size:191;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

// TableName preserves the cross-module import identity table.
func (migration0006ImportRecord) TableName() string { return "module_import_records" }

// applyOptionalModuleRegistry creates only the two logical 0100 tables.
func applyOptionalModuleRegistry(db *gorm.DB) error {
	return withMySQLTableOptions(db).AutoMigrate(&migration0006Configuration{}, &migration0006ImportRecord{})
}
