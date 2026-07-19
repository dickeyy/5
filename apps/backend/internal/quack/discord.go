package quack

import (
	"context"
	"fmt"
	"strings"
)

const (
	permissionAdministrator   uint64 = 1 << 3
	permissionKickMembers     uint64 = 1 << 1
	permissionBanMembers      uint64 = 1 << 2
	permissionManageGuild     uint64 = 1 << 5
	permissionModerateMembers uint64 = 1 << 40
)

// DiscordClient defines the external operations needed by this package, keeping the concrete client at the adapter boundary.
type DiscordClient interface {
	UserGuilds(ctx context.Context, accessToken string) ([]DiscordUserGuild, error)
	BotGuilds(ctx context.Context) ([]DiscordBotGuild, error)
	BotGuild(ctx context.Context, discordGuildID string) (*DiscordBotGuild, error)
	GuildAuthorization(ctx context.Context, discordGuildID, actorDiscordUserID, targetDiscordUserID string) (*DiscordGuildAuthorization, error)
}

// DiscordUserGuild groups the discord user guild state used to keep this package's responsibilities explicit.
type DiscordUserGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon"`
	Owner       bool   `json:"owner"`
	Permissions uint64 `json:"permissions,string"`
}

// DiscordBotGuild groups the discord bot guild state used to keep this package's responsibilities explicit.
type DiscordBotGuild struct {
	ID      string
	Name    string
	Icon    string
	OwnerID string
}

// DiscordGuildAuthorization is a request-scoped snapshot fetched from Discord for one protected operation.
type DiscordGuildAuthorization struct {
	Guild  DiscordBotGuild
	Actor  DiscordMemberAuthorization
	Bot    DiscordMemberAuthorization
	Target *DiscordMemberAuthorization
}

// DiscordMemberAuthorization captures current guild membership, permissions, account type, and hierarchy position.
type DiscordMemberAuthorization struct {
	DiscordUserID   string
	DisplayName     string
	PermissionBits  uint64
	TopRolePosition int
	Present         bool
	Bot             bool
}

// discordGuildIconURL encapsulates the discord guild icon url rule so callers share one consistent package implementation.
func discordGuildIconURL(guildID, iconHash string) string {
	if guildID == "" || iconHash == "" {
		return ""
	}

	ext := "png"
	if strings.HasPrefix(iconHash, "a_") {
		ext = "gif"
	}

	return fmt.Sprintf("https://cdn.discordapp.com/icons/%s/%s.%s", guildID, iconHash, ext)
}
