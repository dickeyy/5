package store

import (
	"strings"
	"testing"
	"time"
)

func TestOptionalModuleLogicalMigrationsJoinCentralLedger(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	if err := New(db, nil).Migrate(); err != nil {
		t.Fatalf("migrate optional modules: %v", err)
	}

	for _, table := range []any{
		&migration0005Configuration{}, &migration0005ImportRecord{},
		&migration0006Transcript{}, &migration0006MemberState{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("central migration did not create %T", table)
		}
	}
	var ledger []schemaMigration
	if err := db.Order("version ASC").Find(&ledger).Error; err != nil {
		t.Fatalf("read migration ledger: %v", err)
	}
	if len(ledger) != 6 || ledger[4].Name != "optional_module_registry_0100" || ledger[5].Name != "ticket_lifecycle_0110" {
		t.Fatalf("unexpected reconciled ledger: %+v", ledger)
	}
	if strings.Contains(migration0005Source, "internal/modules") || strings.Contains(migration0006Source, "internal/modules") {
		t.Fatal("central module migrations depend on mutable feature models")
	}
}

func TestTicketLifecycleMigrationPreservesBaselineTicketRows(t *testing.T) {
	db := openSQLiteMigrationDB(t)
	if err := runMigrations(db, registeredMigrations()[:5]); err != nil {
		t.Fatalf("apply through module registry: %v", err)
	}
	now := time.Now().UTC()
	want := migration0006Ticket{
		ID: "01J50000000000000000000001", GuildID: "01J50000000000000000000002",
		OwnerDiscordUserID: "member", ThreadDiscordChannelID: "channel",
		Status: "resolved", MetadataJSON: `{"legacy":true}`, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&want).Error; err != nil {
		t.Fatalf("insert baseline ticket: %v", err)
	}
	if err := runMigrations(db, registeredMigrations()); err != nil {
		t.Fatalf("apply ticket lifecycle: %v", err)
	}
	var got migration0006Ticket
	if err := db.First(&got, "id = ?", want.ID).Error; err != nil {
		t.Fatalf("load preserved ticket: %v", err)
	}
	if got.GuildID != want.GuildID || got.Status != want.Status || got.MetadataJSON != want.MetadataJSON {
		t.Fatalf("ticket changed during lifecycle migration: got=%+v want=%+v", got, want)
	}
}
