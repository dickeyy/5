package store

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// openSQLiteMigrationDB creates an isolated SQLite database for migration tests.
func openSQLiteMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:migration-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite migration database: %v", err)
	}
	return db
}

func TestMigrateSQLiteForwardAndRerun(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	repositories := New(db, nil)

	if err := repositories.Migrate(); err != nil {
		t.Fatalf("migrate clean sqlite schema: %v", err)
	}
	if err := repositories.Migrate(); err != nil {
		t.Fatalf("rerun sqlite migrations: %v", err)
	}

	for _, table := range []string{"guilds", "cases", "case_action_attempts", "case_events", "audit_log_entries", "quack_schema_migrations"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %s", table)
		}
	}
	var ledgerCount int64
	if err := db.Model(&schemaMigration{}).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count migration ledger: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("expected one migration ledger row after rerun, got %d", ledgerCount)
	}
}

func TestRegisteredMigrationsProduceEveryCurrentSchemaFieldAndIndex(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	if err := New(db, nil).Migrate(); err != nil {
		t.Fatalf("migrate clean sqlite schema: %v", err)
	}

	for _, currentModel := range schemaModels() {
		statement := &gorm.Statement{DB: db}
		if err := statement.Parse(currentModel); err != nil {
			t.Fatalf("parse current schema model %T: %v", currentModel, err)
		}
		if !db.Migrator().HasTable(currentModel) {
			t.Fatalf("registered migrations did not create %s", statement.Schema.Table)
		}
		for _, field := range statement.Schema.Fields {
			if field.DBName != "" && !db.Migrator().HasColumn(currentModel, field.DBName) {
				t.Fatalf("registered migrations did not create %s.%s", statement.Schema.Table, field.DBName)
			}
		}
		for _, index := range statement.Schema.ParseIndexes() {
			if !db.Migrator().HasIndex(currentModel, index.Name) {
				t.Fatalf("registered migrations did not create index %s on %s", index.Name, statement.Schema.Table)
			}
		}
	}
}

func TestMigrateAdoptsCurrentSchemaWithoutLosingHistory(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	if err := applyInitialV5Schema(db); err != nil {
		t.Fatalf("create representative pre-ledger schema: %v", err)
	}
	want := insertRepresentativeHistory(t, db)

	repositories := New(db, nil)
	if err := repositories.Migrate(); err != nil {
		t.Fatalf("adopt representative schema: %v", err)
	}
	if err := repositories.Migrate(); err != nil {
		t.Fatalf("rerun representative schema migration: %v", err)
	}
	assertRepresentativeHistory(t, db, want)
}

func TestMigrateAddsKnownCurrentV5ColumnsToOlderSchema(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	if err := db.Exec(`CREATE TABLE schema_migrations (name text primary key, applied_at datetime not null)`).Error; err != nil {
		t.Fatalf("create obsolete migration table: %v", err)
	}
	if err := db.Exec(`INSERT INTO schema_migrations (name, applied_at) VALUES ("0001_v5_schema", CURRENT_TIMESTAMP)`).Error; err != nil {
		t.Fatalf("insert obsolete migration row: %v", err)
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
		t.Fatalf("create older case_template_levels: %v", err)
	}

	if err := New(db, nil).Migrate(); err != nil {
		t.Fatalf("migrate older schema: %v", err)
	}
	if !db.Migrator().HasColumn("case_template_levels", "notify_user") {
		t.Fatal("expected notify_user column to be added")
	}
	if !db.Migrator().HasColumn("case_template_levels", "notification_type") {
		t.Fatal("expected notification_type column to be added")
	}
	if !db.Migrator().HasTable("schema_migrations") {
		t.Fatal("expected obsolete ledger to remain untouched")
	}
}

func TestMigrateRejectsEditedAppliedMigration(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	repositories := New(db, nil)
	if err := repositories.Migrate(); err != nil {
		t.Fatalf("migrate sqlite schema: %v", err)
	}
	if err := db.Model(&schemaMigration{}).Where("version = ?", 1).Update("checksum", "edited").Error; err != nil {
		t.Fatalf("tamper migration checksum: %v", err)
	}

	err := repositories.Migrate()
	if !errors.Is(err, ErrMigrationChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestMigrationFailureCanRerunAndDoesNotRecordSuccessEarly(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	failed := []migration{{
		Version: 1, Name: "recoverable", Definition: "create recoverable_table",
		Up: func(tx *gorm.DB) error { return errors.New("injected failure") },
	}}
	if err := runMigrations(db, failed); err == nil {
		t.Fatal("expected injected migration failure")
	}
	var ledgerCount int64
	if err := db.Model(&schemaMigration{}).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count ledger after failure: %v", err)
	}
	if ledgerCount != 0 {
		t.Fatalf("expected no ledger row after failure, got %d", ledgerCount)
	}

	recovered := []migration{{
		Version: 1, Name: "recoverable", Definition: "create recoverable_table",
		Up: func(tx *gorm.DB) error {
			return tx.Exec("CREATE TABLE IF NOT EXISTS recoverable_table (id integer primary key)").Error
		},
	}}
	if err := runMigrations(db, recovered); err != nil {
		t.Fatalf("rerun recovered migration: %v", err)
	}
	if !db.Migrator().HasTable("recoverable_table") {
		t.Fatal("expected recovered migration table")
	}
}

func TestRollbackLastMigrationRunsReviewedInverse(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	migrations := []migration{{
		Version: 1, Name: "reversible", Definition: "create reversible_table; down drops only that empty table",
		Up: func(tx *gorm.DB) error {
			return tx.Exec("CREATE TABLE reversible_table (id integer primary key)").Error
		},
		Down: func(tx *gorm.DB) error { return tx.Exec("DROP TABLE reversible_table").Error },
	}}
	if err := runMigrations(db, migrations); err != nil {
		t.Fatalf("apply reversible migration: %v", err)
	}
	if err := rollbackLastMigration(db, migrations); err != nil {
		t.Fatalf("roll back reversible migration: %v", err)
	}
	if db.Migrator().HasTable("reversible_table") {
		t.Fatal("expected reversible table to be removed")
	}
	var ledgerCount int64
	if err := db.Model(&schemaMigration{}).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count ledger after rollback: %v", err)
	}
	if ledgerCount != 0 {
		t.Fatalf("expected rollback to remove ledger row, got %d", ledgerCount)
	}
}

func TestRollbackRefusesForwardOnlyBaselineWithoutChangingHistory(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	repositories := New(db, nil)
	if err := repositories.Migrate(); err != nil {
		t.Fatalf("migrate baseline: %v", err)
	}
	want := insertRepresentativeHistory(t, db)

	err := repositories.RollbackLastMigration()
	if !errors.Is(err, ErrMigrationNotReversible) {
		t.Fatalf("expected forward-only refusal, got %v", err)
	}
	assertRepresentativeHistory(t, db, want)
}

// representativeHistory identifies immutable moderation records expected to survive migration operations.
type representativeHistory struct {
	GuildID, CaseID, AttemptID, EventID, AuditID string
	CaseNumber                                   uint64
	TemplateSnapshot                             string
}

// insertRepresentativeHistory inserts IDs, numbering, snapshots, attempts, events, and audit rows from the current v5 shape.
func insertRepresentativeHistory(t *testing.T, db *gorm.DB) representativeHistory {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	want := representativeHistory{
		GuildID: "01J00000000000000000000001", CaseID: "01J00000000000000000000002",
		AttemptID: "01J00000000000000000000004", EventID: "01J00000000000000000000005",
		AuditID: "01J00000000000000000000006", CaseNumber: 42, TemplateSnapshot: `{"template":"preserved"}`,
	}
	records := []any{
		&GuildRecord{ULIDModelRecord: ULIDModelRecord{ID: want.GuildID, CreatedAt: now, UpdatedAt: now}, DiscordGuildID: "123", Name: "Fixture Guild", OwnerDiscordUserID: "owner"},
		&CaseRecord{ULIDModelRecord: ULIDModelRecord{ID: want.CaseID, CreatedAt: now, UpdatedAt: now}, GuildID: want.GuildID, CaseNumber: want.CaseNumber, TemplateVersion: 3, TemplateSnapshotJSON: want.TemplateSnapshot, TargetDiscordUserID: "target", ModeratorDiscordUserID: "moderator", Reason: "preserve", Severity: model.CaseSeverity("medium"), Weight: 1, Status: model.CaseStatus("open"), Source: model.CaseSource("discord_command"), MetadataJSON: "{}"},
		&CaseActionExecutionRecord{ULIDModelRecord: ULIDModelRecord{ID: "01J00000000000000000000003", CreatedAt: now, UpdatedAt: now}, CaseID: want.CaseID, Position: 1, ActionType: model.ActionType("send_dm"), Status: model.ActionExecutionStatus("succeeded"), IdempotencyKey: "fixture-action", ConfigSnapshotJSON: "{}"},
		&CaseActionAttemptRecord{ULIDModelRecord: ULIDModelRecord{ID: want.AttemptID, CreatedAt: now, UpdatedAt: now}, ExecutionID: "01J00000000000000000000003", AttemptNumber: 1, Status: model.ActionAttemptStatus("succeeded"), StartedAt: now, RequestPayloadJSON: "{}", ResponsePayloadJSON: "{}"},
		&CaseEventRecord{ULIDModelRecord: ULIDModelRecord{ID: want.EventID, CreatedAt: now, UpdatedAt: now}, CaseID: want.CaseID, GuildID: want.GuildID, EventType: model.CaseEventType("created"), ActorType: "staff", Visibility: model.EventVisibility("staff"), Body: "fixture event", MetadataJSON: "{}"},
		&AuditLogEntryRecord{ULIDModelRecord: ULIDModelRecord{ID: want.AuditID, CreatedAt: now, UpdatedAt: now}, GuildID: want.GuildID, Source: model.AuditSource("dashboard"), Action: "case.create", ResourceType: "case", ResourceID: want.CaseID, Result: model.AuditResult("success"), MetadataJSON: "{}"},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("insert representative history %T: %v", record, err)
		}
	}
	return want
}

// assertRepresentativeHistory verifies the immutable identifiers and snapshots required by v5 history rules.
func assertRepresentativeHistory(t *testing.T, db *gorm.DB, want representativeHistory) {
	t.Helper()
	var guild GuildRecord
	if err := db.First(&guild, "id = ?", want.GuildID).Error; err != nil {
		t.Fatalf("load preserved guild: %v", err)
	}
	var caseRecord CaseRecord
	if err := db.First(&caseRecord, "id = ?", want.CaseID).Error; err != nil {
		t.Fatalf("load preserved case: %v", err)
	}
	if caseRecord.CaseNumber != want.CaseNumber || caseRecord.TemplateSnapshotJSON != want.TemplateSnapshot {
		t.Fatalf("case history changed: number=%d snapshot=%s", caseRecord.CaseNumber, caseRecord.TemplateSnapshotJSON)
	}
	for label, query := range map[string]any{
		"attempt": &CaseActionAttemptRecord{ULIDModelRecord: ULIDModelRecord{ID: want.AttemptID}},
		"event":   &CaseEventRecord{ULIDModelRecord: ULIDModelRecord{ID: want.EventID}},
		"audit":   &AuditLogEntryRecord{ULIDModelRecord: ULIDModelRecord{ID: want.AuditID}},
	} {
		if err := db.First(query).Error; err != nil {
			t.Fatalf("load preserved %s: %v", label, err)
		}
	}
}
