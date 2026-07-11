package store

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

const mysqlTableOptions = "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci"

// Migrate applies the existing AutoMigrate model set while preserving all legacy table options and names.
func (s *Store) Migrate() error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}

	models := schemaModels()
	if len(models) == 0 {
		return errors.New("no schema models registered")
	}

	if err := withMySQLTableOptions(s.db).AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migrate schema: %w", err)
	}
	return nil
}

// withMySQLTableOptions encapsulates the with my sqltable options rule so callers share one consistent package implementation.
func withMySQLTableOptions(db *gorm.DB) *gorm.DB {
	if db.Dialector.Name() != "mysql" {
		return db
	}

	return db.Set("gorm:table_options", mysqlTableOptions)
}
