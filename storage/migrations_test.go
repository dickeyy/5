package storage_test

import (
	"testing"

	"github.com/quackdiscord/bot/internal/testutil"
)

func TestMigrateSQLiteSmoke(t *testing.T) {
	store := testutil.NewSQLiteStore(t)

	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate sqlite schema: %v", err)
	}
}

func TestMigrateSyncsExistingSchema(t *testing.T) {
	store := testutil.NewSQLiteStore(t)
	db := store.DB()

	// Existing dev databases may still have this table from the old wrapper.
	// Migrate should ignore it and sync the live GORM models.
	if err := db.Exec(`CREATE TABLE schema_migrations (name text primary key, applied_at datetime not null)`).Error; err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	if err := db.Exec(`INSERT INTO schema_migrations (name, applied_at) VALUES ("0001_v5_schema", CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("insert initial migration: %v", err)
	}
	if err := db.Exec(`CREATE TABLE case_template_levels (
		id text primary key,
		created_at datetime not null,
		updated_at datetime not null,
		template_id text not null,
		position integer not null,
		name text not null,
		is_default numeric not null,
		trigger_case_count integer not null,
		window_minutes integer not null,
		enabled numeric not null
	)`).Error; err != nil {
		t.Fatalf("create old case_template_levels: %v", err)
	}

	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate old schema: %v", err)
	}
	if !db.Migrator().HasColumn("case_template_levels", "notify_user") {
		t.Fatalf("expected notify_user column to be added")
	}
	if !db.Migrator().HasColumn("case_template_levels", "notification_type") {
		t.Fatalf("expected notification_type column to be added")
	}
}
