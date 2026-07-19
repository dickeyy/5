package store

import (
	"fmt"

	mysqlconfig "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// OpenMySQL opens and verifies my sql so startup fails before serving traffic when the dependency is unavailable.
func OpenMySQL(dsn string) (*gorm.DB, error) {
	normalized, err := normalizeMySQLDSN(dsn)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(gormmysql.Open(normalized), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql database: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

// normalizeMySQLDSN produces a stable my sqldsn representation for deterministic validation, comparison, or caching.
func normalizeMySQLDSN(dsn string) (string, error) {
	cfg, err := mysqlconfig.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("parse mysql dsn: %w", err)
	}
	cfg.ParseTime = true
	return cfg.FormatDSN(), nil
}
