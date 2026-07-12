package store

import _ "embed"

// migration0001Source binds migration 0001's executable logic and frozen schema to its ledger checksum.
//
//go:embed migration_0001_initial_v5_schema.go
var migration0001Source string

// migration0002Source binds migration 0002's executable compatibility logic to its ledger checksum.
//
//go:embed migration_0002_simplify_template_model.go
var migration0002Source string

// migration0003Source binds migration 0003's case validity compatibility logic to its ledger checksum.
//
//go:embed migration_0003_case_validity.go
var migration0003Source string

// registeredMigrations returns the immutable ordered production migration registry.
func registeredMigrations() []migration {
	return []migration{
		migration0001InitialV5Schema(),
		migration0002SimplifyTemplateModel(),
		migration0003CaseValidity(),
	}
}
