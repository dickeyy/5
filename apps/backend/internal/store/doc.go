// Package store implements Quack's persistence ports using MySQL and Redis.
//
// Moderation changes and their audit evidence commit together. Cases and audit
// entries are never hard-deleted by normal product operations. Action and
// notification leases fence stale workers; a lease cannot prove whether an
// external Discord request succeeded. Migrations preserve historical v4 records
// without allowing them to contribute to v5 escalation.
package store
