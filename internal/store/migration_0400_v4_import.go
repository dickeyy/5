package store

import "gorm.io/gorm"

const migration0400Definition = `v4-historical-import-v1
logical migration: 0400 v4_historical_import
schema: privacy-safe batch ledger and one immutable source-to-historical-case mapping per guild, source, and source id
invariants: imported cases are ordinary readable history with source v4_import; ledgers contain checksums and counts, never member content
rollback: forward-only; operators use the guarded import rollback command for untouched imported batches`

// migration0400V4HistoricalImport returns the logical 0400 migration at an integration-assigned contiguous ledger version.
func migration0400V4HistoricalImport(version uint64) migration {
	return migration{Version: version, Name: "v4_historical_import_0400", Definition: migration0400Definition, Source: migration0400Source, Up: func(db *gorm.DB) error {
		return withMySQLTableOptions(db).AutoMigrate(&V4ImportBatchRecord{}, &V4ImportSourceRecord{})
	}}
}
