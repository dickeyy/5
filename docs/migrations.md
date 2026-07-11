# Database Migrations

Production startup applies an ordered migration registry from `internal/store`.
Each successful migration is recorded in `quack_schema_migrations` with its
version, name, checksum, and application time. Startup verifies every existing
ledger checksum before it runs new work. The checksum includes an embedded copy
of the migration's Go source, including its frozen schema records and `Up`/`Down`
logic, so executable edits cannot retain the old identity. There is no
production or development `AutoMigrate` path.

Migration definitions are additive by default and must preserve table names,
identifiers, guild case numbers, snapshots, action attempts, events, and audit
rows. The first migration adopts the current pre-ledger v5 schema or creates it
on a clean database. It never drops or renames an application table or column.

## Forward procedure

1. Back up MySQL and verify the backup before deploying schema-changing code.
2. Review the ordered migration definition, embedded source, `Up` operation,
   and, when safe, its idempotent `Down` operation in the pull request.
3. Stop additional Quack processes or leave them waiting on the MySQL migration
   lock. Run `go run ./cmd/quack-migrate up` with the production
   `DATABASE_DSN`, or start one new Quack process and let startup run the same
   method.
4. Verify the command succeeds and inspect `quack_schema_migrations`. Do not
   manually insert, delete, rename, or edit ledger rows.
5. Start the remaining processes and verify readiness before normal traffic.

Rerunning `up` is expected and safe. Applied migrations are checksum-verified
and skipped. A failed migration is not recorded as successful. MySQL may commit
DDL even when a later statement fails, so every `Up` operation must detect its
already-applied additive work and safely resume on the next run.

## Failure and recovery procedure

1. Keep the application unavailable when the new binary requires the failed
   schema transition.
2. Preserve the error and current ledger; never mark a failed migration applied
   by hand.
3. Inspect the database because MySQL DDL can survive a failed transaction.
4. Fix the migration so its reviewed `Up` operation resumes from that state,
   then rerun `go run ./cmd/quack-migrate up`.
5. Restore the verified backup only when additive recovery cannot preserve the
   required records, and rehearse that restore before production use.

## Rollback procedure

Run `go run ./cmd/quack-migrate down` only after reviewing the newest applied
migration and confirming its idempotent `Down` operation preserves v5 history.
Before executing MySQL DDL, the runner durably marks the ledger row
`rolling_back`. Normal `up` and application startup refuse that dirty state.
After `Down` succeeds, the runner removes the ledger row. If the process or DDL
fails before removal, rerun the same reviewed `down`; it resumes the idempotent
inverse and completes ledger cleanup. Never clear `rolling_back` by hand. Tests
cover partial-DDL recovery and reversible rollback on SQLite and MySQL.

Migrations without a safe inverse are explicitly forward-only. The initial v5
baseline is forward-only because reversing it would hard-delete moderation and
audit history; `down` returns `ErrMigrationNotReversible` and changes neither
the schema nor ledger. For an additive forward-only release, roll back the
application binary while retaining the compatible schema, then ship a new
forward migration for any database correction.

Every future schema change must live in a new source file embedded by its new
registry entry. Do not edit an applied migration. Its focused tests must cover
forward execution, rerun behavior, failure recovery, preservation of
representative data, and either an idempotent reviewed inverse with dirty-state
recovery or the explicit forward-only refusal boundary.
