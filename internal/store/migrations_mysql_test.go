package store

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	mysqlconfig "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMySQLMigrateForwardRerunPreservationAndRollbackBoundary(t *testing.T) {
	db := openMySQLMigrationDB(t)
	if err := applyInitialV5Schema(db); err != nil {
		t.Fatalf("create representative pre-ledger MySQL schema: %v", err)
	}
	want := insertRepresentativeHistory(t, db)
	disabledTemplateID := insertMigration0002Template(t, db, "mysql-disabled", false, 0)
	windowTemplateID := insertMigration0002Template(t, db, "mysql-window", true, 60)
	deletedTemplateID := insertMigration0002Template(t, db, "mysql-deleted", true, 0)
	if err := db.Model(&CaseTemplateRecord{}).Where("id = ?", deletedTemplateID).UpdateColumn("deleted_at", time.Now().UTC()).Error; err != nil {
		t.Fatalf("soft delete MySQL compatibility fixture: %v", err)
	}
	repositories := New(db, nil)

	if err := repositories.Migrate(); err != nil {
		t.Fatalf("adopt representative MySQL schema: %v", err)
	}
	if err := repositories.Migrate(); err != nil {
		t.Fatalf("rerun MySQL migrations: %v", err)
	}
	assertRepresentativeHistory(t, db, want)
	assertTemplateArchiveState(t, db, disabledTemplateID, true)
	assertTemplateArchiveState(t, db, windowTemplateID, true)
	assertTemplateArchiveState(t, db, deletedTemplateID, true)
	assertTemplateDeletedState(t, db, deletedTemplateID, false)

	if err := repositories.RollbackLastMigration(); err != nil {
		t.Fatalf("roll back template compatibility migration: %v", err)
	}
	assertTemplateArchiveState(t, db, disabledTemplateID, false)
	assertTemplateArchiveState(t, db, windowTemplateID, false)
	assertTemplateArchiveState(t, db, deletedTemplateID, false)
	assertTemplateDeletedState(t, db, deletedTemplateID, true)
	err := repositories.RollbackLastMigration()
	if !errors.Is(err, ErrMigrationNotReversible) {
		t.Fatalf("expected baseline rollback refusal, got %v", err)
	}
	assertRepresentativeHistory(t, db, want)
}

func TestMySQLMigrationRecoversFromPartialDDLAndRunsReviewedRollback(t *testing.T) {
	db := openMySQLMigrationDB(t)
	baseline := migration0001InitialV5Schema()
	failDownOnce := true
	failed := []migration{
		baseline,
		{
			Version: 2, Name: "mysql_recovery_probe", Definition: "create migration_recovery_probe; down drops only the probe table",
			Source: "up: create migration_recovery_probe if absent; down: drop migration_recovery_probe if present",
			Up: func(tx *gorm.DB) error {
				if err := tx.Exec("CREATE TABLE IF NOT EXISTS migration_recovery_probe (id bigint primary key)").Error; err != nil {
					return err
				}
				return errors.New("injected post-DDL failure")
			},
			Down: func(tx *gorm.DB) error {
				if err := tx.Exec("DROP TABLE IF EXISTS migration_recovery_probe").Error; err != nil {
					return err
				}
				if failDownOnce {
					failDownOnce = false
					return errors.New("injected failure after MySQL DDL")
				}
				return nil
			},
		},
	}
	if err := runMigrations(db, failed); err == nil {
		t.Fatal("expected injected MySQL migration failure")
	}
	var ledgerCount int64
	if err := db.Model(&schemaMigration{}).Count(&ledgerCount).Error; err != nil {
		t.Fatalf("count MySQL ledger after failure: %v", err)
	}
	if ledgerCount != 1 {
		t.Fatalf("expected only baseline ledger row after failure, got %d", ledgerCount)
	}
	if !db.Migrator().HasTable("migration_recovery_probe") {
		t.Fatal("expected MySQL implicit DDL commit to leave probe table for idempotent recovery")
	}

	recovered := append([]migration(nil), failed...)
	recovered[1].Up = func(tx *gorm.DB) error {
		return tx.Exec("CREATE TABLE IF NOT EXISTS migration_recovery_probe (id bigint primary key)").Error
	}
	if err := runMigrations(db, recovered); err != nil {
		t.Fatalf("recover partial MySQL migration: %v", err)
	}
	if err := rollbackLastMigration(db, recovered); err == nil {
		t.Fatal("expected injected partial MySQL rollback failure")
	}
	if db.Migrator().HasTable("migration_recovery_probe") {
		t.Fatal("expected partial MySQL rollback to have committed the probe-table drop")
	}
	if err := runMigrations(db, recovered); !errors.Is(err, ErrMigrationDirty) {
		t.Fatalf("expected MySQL startup to refuse dirty rollback, got %v", err)
	}
	var dirty schemaMigration
	if err := db.First(&dirty, "version = ?", 2).Error; err != nil {
		t.Fatalf("load dirty MySQL migration row: %v", err)
	}
	if dirty.State != migrationStateRollingBack || dirty.RollbackStartedAt == nil {
		t.Fatalf("expected durable MySQL rolling_back state, got state=%q started=%v", dirty.State, dirty.RollbackStartedAt)
	}

	if err := rollbackLastMigration(db, recovered); err != nil {
		t.Fatalf("recover partial MySQL rollback: %v", err)
	}
	if err := runMigrations(db, recovered); err != nil {
		t.Fatalf("reapply MySQL migration after recovered rollback: %v", err)
	}
}

// openMySQLMigrationDB creates and later drops a dedicated database so integration tests never alter operator data.
func openMySQLMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("QUACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("QUACK_TEST_MYSQL_DSN is not configured")
	}
	cfg, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("parse QUACK_TEST_MYSQL_DSN: %v", err)
	}
	cfg.ParseTime = true
	databaseName := fmt.Sprintf("quack_migration_%d", time.Now().UTC().UnixNano())
	adminCfg := *cfg
	adminCfg.DBName = ""
	adminDB, err := gorm.Open(gormmysql.Open(adminCfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatalf("open MySQL admin connection: %v", err)
	}
	if err := adminDB.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		t.Fatalf("create isolated MySQL migration database: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`").Error; err != nil {
			t.Errorf("drop isolated MySQL migration database: %v", err)
		}
		if sqlDB, err := adminDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	testCfg := *cfg
	testCfg.DBName = databaseName
	db, err := OpenMySQL(testCfg.FormatDSN())
	if err != nil {
		t.Fatalf("open isolated MySQL migration database: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}
