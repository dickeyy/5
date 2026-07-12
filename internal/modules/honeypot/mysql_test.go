package honeypot_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	mysqlconfig "github.com/go-sql-driver/mysql"
	"github.com/quackdiscord/bot/internal/modules"
	"github.com/quackdiscord/bot/internal/modules/honeypot"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestMySQLHoneypotMigration(t *testing.T) {
	dsn := os.Getenv("QUACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("QUACK_TEST_MYSQL_DSN is not configured")
	}
	cfg, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ParseTime = true
	databaseName := fmt.Sprintf("quack_honeypot_%d", time.Now().UTC().UnixNano())
	adminCfg := *cfg
	adminCfg.DBName = ""
	admin, err := gorm.Open(mysql.Open(adminCfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE DATABASE `" + databaseName + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP DATABASE IF EXISTS `" + databaseName + "`").Error; err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})
	testCfg := *cfg
	testCfg.DBName = databaseName
	db, err := gorm.Open(mysql.Open(testCfg.FormatDSN()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range []modules.Migration{modules.RegistryMigration(), honeypot.Migration()} {
		if err := migration.Apply(db); err != nil {
			t.Fatalf("apply %d %s: %v", migration.Version, migration.Name, err)
		}
	}
	for _, table := range []string{"module_configurations", "module_import_records", "honeypot_triggers"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("missing table %s", table)
		}
	}
}
