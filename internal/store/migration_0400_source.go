package store

import _ "embed"

// migration0400Source binds logical migration 0400's executable schema to its ledger checksum.
//
//go:embed migration_0400_v4_import.go
var migration0400Source string
