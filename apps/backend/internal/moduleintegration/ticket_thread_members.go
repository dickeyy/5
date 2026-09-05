package moduleintegration

import (
	"context"
	"errors"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// syncTicketThreadMembers grants current staff access and removes stale invitations.
// Private threads cannot use role overwrites, so their explicit membership follows
// a fresh guild-member snapshot. The owner and bot always retain their invitations.
func (c ticketDiscordClient) syncTicketThreadMembers(ctx context.Context, guildID, threadID, ownerID string, staffRoleIDs []string) error {
	botID, err := c.botUserID(ctx)
	if err != nil {
		return err
	}
	wanted := map[string]struct{}{ownerID: {}, botID: {}}
	roles := make(map[string]struct{}, len(staffRoleIDs))
	for _, roleID := range staffRoleIDs {
		if roleID = strings.TrimSpace(roleID); roleID != "" {
			roles[roleID] = struct{}{}
		}
	}
	after := ""
	for len(roles) > 0 {
		members, err := c.session.GuildMembers(guildID, after, 1000, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
		if err != nil {
			return err
		}
		for _, member := range members {
			if member == nil || member.User == nil || member.User.ID == "" {
				return errors.New("Discord returned an invalid guild member")
			}
			for _, roleID := range member.Roles {
				if _, ok := roles[roleID]; ok {
					wanted[member.User.ID] = struct{}{}
					break
				}
			}
		}
		if len(members) < 1000 {
			break
		}
		next := members[len(members)-1].User.ID
		if next == after {
			return errors.New("Discord repeated a guild-member page")
		}
		after = next
	}
	present := make(map[string]struct{})
	after = ""
	for {
		members, err := c.session.ThreadMembers(threadID, 100, false, after, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false))
		if err != nil {
			return err
		}
		for _, member := range members {
			if member == nil || member.UserID == "" {
				return errors.New("Discord returned an invalid thread member")
			}
			present[member.UserID] = struct{}{}
			if _, ok := wanted[member.UserID]; !ok {
				if err := c.session.ThreadMemberRemove(threadID, member.UserID, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false)); err != nil {
					return err
				}
			}
		}
		if len(members) < 100 {
			break
		}
		next := members[len(members)-1].UserID
		if next == after {
			return errors.New("Discord repeated a thread-member page")
		}
		after = next
	}
	for userID := range wanted {
		if _, ok := present[userID]; ok || userID == botID {
			continue
		}
		if err := c.session.ThreadMemberAdd(threadID, userID, discordgo.WithContext(ctx), discordgo.WithRestRetries(0), discordgo.WithRetryOnRatelimit(false)); err != nil {
			return err
		}
	}
	return nil
}
