package quack

import (
	"context"

	actionmods "github.com/quackdiscord/bot/internal/quack/actionmods"
)

// DiscordActionClient defines the external operations needed by this package, keeping the concrete client at the adapter boundary.
type DiscordActionClient interface {
	SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error)
}

// DiscordActionError carries classified discord action error failure details across package boundaries.
type DiscordActionError = actionmods.DiscordError
