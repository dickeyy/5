package store

import _ "embed"

// migration0001Source binds migration 0001's executable logic and frozen schema to its ledger checksum.
//
//go:embed migration_0001_initial_v5_schema.go
var migration0001Source string

// registeredMigrations returns the immutable ordered production migration registry.
func registeredMigrations() []migration {
	return []migration{
		migration0001InitialV5Schema(),
	}
}
