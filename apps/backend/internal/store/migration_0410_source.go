package store

import _ "embed"

// migration0410Source binds logical migration 0410's executable schema to its ledger checksum.
//
//go:embed migration_0410_final_constraints.go
var migration0410Source string
