package storage

import (
	"errors"
	"fmt"

	"github.com/quackdiscord/bot/structs"
	"gorm.io/gorm"
)

const mysqlTableOptions = "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"

func (s *Store) Migrate() error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	models := structs.SchemaModels()
	if len(models) == 0 {
		return errors.New("no schema models registered")
	}

	if err := withMySQLTableOptions(s.db).AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate schema: %w", err)
	}
	return nil
}

func withMySQLTableOptions(db *gorm.DB) *gorm.DB {
	if db.Dialector.Name() != "mysql" {
		return db
	}

	return db.Set("gorm:table_options", mysqlTableOptions)
}
