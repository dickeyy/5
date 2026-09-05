// Package quack implements the moderation behavior shared by Discord and HTTP.
//
// Templates own policy; case creation snapshots that policy after live Discord
// authorization. A guild-scoped transaction serializes numbering and escalation.
// Persisted action rows are the durable work queue; in-memory submissions only
// reduce latency. Services accept consumer-defined persistence ports and keep
// transport responses separate from database records.
package quack
