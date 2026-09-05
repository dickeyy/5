// Package moduleintegration composes optional modules with Discord and HTTP.
//
// Tickets, general logging, and honeypots own their domain state. This package
// translates shared guild identities, permissions, and gateway events into their
// narrow ports. Each background queue has its own capacity and shutdown lifecycle
// so optional traffic cannot consume the moderation action queue.
package moduleintegration
