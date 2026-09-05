// Package discordbot adapts Discord's gateway and REST API to Quack services.
//
// Live REST reads establish current authorization; gateway caches support
// displays only. REST calls carry caller contexts and leave recorded retries to
// Quack's workers. Commands, components, and views live in their own packages so
// transport presentation does not define moderation policy.
package discordbot
