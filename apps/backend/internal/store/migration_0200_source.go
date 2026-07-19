package store

import _ "embed"

// migration0200Source binds logical migration 0200's executable schema to its ledger checksum.
//
//go:embed migration_0200_appeals.go
var migration0200Source string
