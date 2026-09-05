package discordbot

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
)

// GuildAuthorization fetches current guild, actor, bot, and optional target state directly from Discord for one protected request.
func (b *Bot) GuildAuthorization(ctx context.Context, guildID, actorDiscordUserID, targetDiscordUserID string) (*quack.DiscordGuildAuthorization, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.Session == nil {
		return nil, quack.ErrAuthorizationUnavailable
	}
	guild, err := b.Session.Guild(guildID, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
	if err != nil {
		return nil, guildAuthorizationError(err)
	}
	if guild == nil {
		return nil, quack.ErrAuthorizationUnavailable
	}

	botID := ""
	if b.Session.State != nil && b.Session.State.User != nil {
		botID = b.Session.State.User.ID
	}
	if botID == "" {
		botUser, userErr := b.Session.User("@me", discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
		if userErr != nil || botUser == nil {
			return nil, quack.ErrAuthorizationUnavailable
		}
		botID = botUser.ID
	}

	actor, err := b.liveMemberAuthorization(ctx, guild, actorDiscordUserID)
	if err != nil {
		return nil, err
	}
	bot, err := b.liveMemberAuthorization(ctx, guild, botID)
	if err != nil {
		return nil, err
	}
	snapshot := &quack.DiscordGuildAuthorization{Guild: *botGuild(guild), Actor: actor, Bot: bot}
	if strings.TrimSpace(targetDiscordUserID) != "" {
		target, targetErr := b.liveMemberAuthorization(ctx, guild, targetDiscordUserID)
		if targetErr != nil {
			return nil, targetErr
		}
		snapshot.Target = &target
	}
	return snapshot, nil
}

// guildAuthorizationError preserves inactive-guild semantics while hiding transient Discord failures.
func guildAuthorizationError(err error) error {
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil {
		switch restErr.Response.StatusCode {
		case http.StatusForbidden, http.StatusNotFound:
			return quack.ErrBotNotInGuild
		}
	}
	return quack.ErrAuthorizationUnavailable
}

// liveMemberAuthorization fetches one current member and safely represents Discord's unknown-member response as non-membership.
func (b *Bot) liveMemberAuthorization(ctx context.Context, guild *discordgo.Guild, userID string) (quack.DiscordMemberAuthorization, error) {
	memberState := quack.DiscordMemberAuthorization{DiscordUserID: strings.TrimSpace(userID)}
	if memberState.DiscordUserID == "" {
		return memberState, nil
	}
	if err := ctx.Err(); err != nil {
		return memberState, err
	}
	member, err := b.Session.GuildMember(guild.ID, memberState.DiscordUserID, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
	if err != nil {
		var restErr *discordgo.RESTError
		if errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound {
			return memberState, nil
		}
		return memberState, quack.ErrAuthorizationUnavailable
	}
	return discordMemberAuthorization(guild, member), nil
}

// discordMemberAuthorization calculates guild-level permission and hierarchy state from a fresh guild/member response.
func discordMemberAuthorization(guild *discordgo.Guild, member *discordgo.Member) quack.DiscordMemberAuthorization {
	if guild == nil || member == nil || member.User == nil {
		return quack.DiscordMemberAuthorization{}
	}
	permissions := int64(0)
	topRolePosition := 0
	roleIDs := make(map[string]struct{}, len(member.Roles))
	for _, roleID := range member.Roles {
		roleIDs[roleID] = struct{}{}
	}
	for _, role := range guild.Roles {
		if role == nil {
			continue
		}
		if role.ID == guild.ID {
			permissions |= role.Permissions
		}
		if _, ok := roleIDs[role.ID]; ok {
			permissions |= role.Permissions
			if role.Position > topRolePosition {
				topRolePosition = role.Position
			}
		}
	}
	if member.User.ID == guild.OwnerID || permissions&discordgo.PermissionAdministrator != 0 {
		permissions |= discordgo.PermissionAll
	}
	displayName := strings.TrimSpace(member.Nick)
	if displayName == "" {
		displayName = strings.TrimSpace(member.User.GlobalName)
	}
	if displayName == "" {
		displayName = member.User.Username
	}
	return quack.DiscordMemberAuthorization{
		DiscordUserID: member.User.ID, DisplayName: displayName,
		PermissionBits: uint64(permissions), TopRolePosition: topRolePosition,
		Present: true, Bot: member.User.Bot,
	}
}
