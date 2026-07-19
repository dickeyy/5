package moduleintegration

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/modules/generallogging"
	"github.com/quackdiscord/bot/internal/modules/tickets"
)

const ticketChannelPermissions = discordgo.PermissionViewChannel |
	discordgo.PermissionSendMessages |
	discordgo.PermissionReadMessageHistory |
	discordgo.PermissionAttachFiles |
	discordgo.PermissionEmbedLinks

// ticketDiscordClient implements the module's narrow private-channel port with
// Discord resources resolved from the integration-owned guild adapter.
type ticketDiscordClient struct {
	session  *discordgo.Session
	resolver guildResolver
}

// CreatePrivateTicketChannel provisions either a private thread under the
// configured entry channel or a private text channel with explicit ACLs.
func (c ticketDiscordClient) CreatePrivateTicketChannel(ctx context.Context, guildID, ownerID string, settings tickets.Settings) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	discordGuildID, err := c.resolver.discordID(ctx, guildID)
	if err != nil {
		return "", err
	}
	entryChannel, err := c.session.Channel(settings.EntryChannelDiscordID)
	if err != nil {
		return "", err
	}
	if entryChannel.GuildID != discordGuildID {
		return "", errors.New("ticket entry channel belongs to another guild")
	}
	name := "ticket-" + ticketNameSuffix(ownerID)
	if settings.UsePrivateThreads {
		thread, err := c.session.ThreadStartComplex(settings.EntryChannelDiscordID, &discordgo.ThreadStart{
			Name: name, Type: discordgo.ChannelTypeGuildPrivateThread,
			AutoArchiveDuration: 1440, Invitable: false,
		})
		if err != nil {
			return "", err
		}
		if err := c.session.ThreadMemberAdd(thread.ID, ownerID); err != nil {
			_, _ = c.session.ChannelDelete(thread.ID)
			return "", err
		}
		return thread.ID, nil
	}

	botID, err := c.botUserID()
	if err != nil {
		return "", err
	}
	overwrites := ticketPermissionOverwrites(discordGuildID, ownerID, botID, settings.StaffRoleDiscordIDs)
	channel, err := c.session.GuildChannelCreateComplex(discordGuildID, discordgo.GuildChannelCreateData{
		Name: name, Type: discordgo.ChannelTypeGuildText,
		ParentID: entryChannel.ParentID, PermissionOverwrites: overwrites,
	})
	if err != nil {
		return "", err
	}
	return channel.ID, nil
}

// EnsureTicketPermissions validates private-thread inheritance or replaces a
// text channel's ACL with owner, configured staff, and bot-only visibility.
func (c ticketDiscordClient) EnsureTicketPermissions(ctx context.Context, channelID, ownerID, guildID string, staffRoleIDs []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	discordGuildID, err := c.resolver.discordID(ctx, guildID)
	if err != nil {
		return err
	}
	channel, err := c.session.Channel(channelID)
	if err != nil {
		return err
	}
	if channel.IsThread() {
		if channel.Type != discordgo.ChannelTypeGuildPrivateThread {
			return errors.New("ticket thread is not private")
		}
		if err := c.session.ThreadMemberAdd(channelID, ownerID); err != nil {
			return err
		}
		return c.addTicketStaffMembers(ctx, discordGuildID, channelID, staffRoleIDs)
	}
	botID, err := c.botUserID()
	if err != nil {
		return err
	}
	updated, err := c.session.ChannelEditComplex(channelID, &discordgo.ChannelEdit{
		PermissionOverwrites: ticketPermissionOverwrites(discordGuildID, ownerID, botID, staffRoleIDs),
	})
	if err != nil {
		return err
	}
	return validateTicketACL(updated, discordGuildID, ownerID, botID, staffRoleIDs)
}

// botUserID returns the current application identity needed for explicit ACLs.
func (c ticketDiscordClient) botUserID() (string, error) {
	if c.session.State != nil && c.session.State.User != nil && c.session.State.User.ID != "" {
		return c.session.State.User.ID, nil
	}
	user, err := c.session.User("@me")
	if err != nil {
		return "", err
	}
	if user == nil || user.ID == "" {
		return "", errors.New("Discord bot identity is unavailable")
	}
	return user.ID, nil
}

// addTicketStaffMembers grants configured staff-role members explicit private
// thread membership because Discord threads cannot accept role overwrites.
func (c ticketDiscordClient) addTicketStaffMembers(ctx context.Context, guildID, threadID string, staffRoleIDs []string) error {
	roles := make(map[string]struct{}, len(staffRoleIDs))
	for _, roleID := range staffRoleIDs {
		if roleID = strings.TrimSpace(roleID); roleID != "" {
			roles[roleID] = struct{}{}
		}
	}
	after := ""
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		members, err := c.session.GuildMembers(guildID, after, 1000)
		if err != nil {
			return err
		}
		for _, member := range members {
			if member == nil || member.User == nil || !hasConfiguredRole(member.Roles, roles) {
				continue
			}
			if err := c.session.ThreadMemberAdd(threadID, member.User.ID); err != nil {
				return err
			}
		}
		if len(members) < 1000 {
			return nil
		}
		last := members[len(members)-1]
		if last == nil || last.User == nil || last.User.ID == "" {
			return errors.New("Discord returned an invalid guild-member page")
		}
		after = last.User.ID
	}
}

// hasConfiguredRole reports whether one current member belongs to ticket staff.
func hasConfiguredRole(memberRoles []string, configured map[string]struct{}) bool {
	for _, roleID := range memberRoles {
		if _, ok := configured[roleID]; ok {
			return true
		}
	}
	return false
}

// SendTicketReply sends one mention-suppressed message inside the private ticket.
func (c ticketDiscordClient) SendTicketReply(ctx context.Context, channelID, body string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	_, err := c.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: body, AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	return err
}

// CaptureTicketTranscript snapshots the complete available private message
// history in stable chronological order before closure.
func (c ticketDiscordClient) CaptureTicketTranscript(ctx context.Context, channelID string) (string, error) {
	before := ""
	messages := make([]*discordgo.Message, 0, 100)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		page, err := c.session.ChannelMessages(channelID, 100, before, "", "")
		if err != nil {
			return "", err
		}
		messages = append(messages, page...)
		if len(page) < 100 {
			break
		}
		before = page[len(page)-1].ID
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].Timestamp.Before(messages[j].Timestamp) })
	var transcript strings.Builder
	for _, message := range messages {
		authorID := "unknown"
		if message.Author != nil {
			authorID = message.Author.ID
		}
		fmt.Fprintf(&transcript, "[%s] %s: %s\n", message.Timestamp.UTC().Format(time.RFC3339), authorID, message.Content)
		for _, attachment := range message.Attachments {
			fmt.Fprintf(&transcript, "  attachment: %s (%d bytes)\n", attachment.Filename, attachment.Size)
		}
	}
	return transcript.String(), nil
}

// ArchiveTicketChannel archives a thread or deletes a dedicated private text
// channel after its transcript has been durably captured.
func (c ticketDiscordClient) ArchiveTicketChannel(ctx context.Context, channelID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	channel, err := c.session.Channel(channelID)
	if err != nil {
		return err
	}
	if channel.IsThread() {
		archived := true
		locked := true
		_, err = c.session.ChannelEditComplex(channelID, &discordgo.ChannelEdit{Archived: &archived, Locked: &locked})
		return err
	}
	overwrites := make([]*discordgo.PermissionOverwrite, 0, len(channel.PermissionOverwrites))
	for _, existing := range channel.PermissionOverwrites {
		copy := *existing
		if copy.ID != channel.GuildID {
			copy.Allow &^= discordgo.PermissionSendMessages
			copy.Deny |= discordgo.PermissionSendMessages
		}
		overwrites = append(overwrites, &copy)
	}
	name := channel.Name
	if !strings.HasPrefix(name, "closed-") {
		name = "closed-" + name
	}
	_, err = c.session.ChannelEditComplex(channelID, &discordgo.ChannelEdit{Name: name, PermissionOverwrites: overwrites})
	return err
}

// loggingDiscordClient sends already-redacted payloads only to channels whose
// everyone role is denied visibility.
type loggingDiscordClient struct {
	session  *discordgo.Session
	resolver guildResolver
}

// SendStaffLog delivers one mention-suppressed message to a validated channel.
func (c loggingDiscordClient) SendStaffLog(ctx context.Context, guildID, channelID, payload string) error {
	if err := c.ValidateStaffOnlyChannel(ctx, guildID, channelID); err != nil {
		return err
	}
	_, err := c.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content: payload, AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	return err
}

// ValidateStaffOnlyChannel rejects missing, cross-guild, or publicly visible destinations.
func (c loggingDiscordClient) ValidateStaffOnlyChannel(ctx context.Context, guildID, channelID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	discordGuildID, err := c.resolver.discordID(ctx, guildID)
	if err != nil {
		return err
	}
	channel, err := c.session.Channel(channelID)
	if err != nil {
		return err
	}
	if channel.GuildID != discordGuildID {
		return errors.New("logging destination belongs to another guild")
	}
	return c.validateLoggingACL(channel)
}

// validateLoggingACL permits visibility only through current staff-capable
// roles (plus the bot itself) and requires the bot to send successfully.
func (c loggingDiscordClient) validateLoggingACL(channel *discordgo.Channel) error {
	if err := validateStaffOnlyACL(channel, channel.GuildID, nil); err != nil {
		return err
	}
	roles, err := c.session.GuildRoles(channel.GuildID)
	if err != nil {
		return err
	}
	staffRoles := make(map[string]bool, len(roles))
	for _, role := range roles {
		if role == nil {
			continue
		}
		staffRoles[role.ID] = role.Permissions&(discordgo.PermissionAdministrator|discordgo.PermissionManageServer|discordgo.PermissionModerateMembers) != 0
	}
	botID := ""
	if c.session.State != nil && c.session.State.User != nil {
		botID = c.session.State.User.ID
	}
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.Allow&discordgo.PermissionViewChannel == 0 {
			continue
		}
		switch overwrite.Type {
		case discordgo.PermissionOverwriteTypeRole:
			if overwrite.ID != channel.GuildID && !staffRoles[overwrite.ID] {
				return errors.New("logging destination grants a non-staff role visibility")
			}
		case discordgo.PermissionOverwriteTypeMember:
			if overwrite.ID != botID {
				return errors.New("logging destination grants a non-bot member visibility")
			}
		}
	}
	if botID == "" {
		return errors.New("Discord bot identity is unavailable")
	}
	permissions, err := c.session.UserChannelPermissions(botID, channel.ID)
	if err != nil {
		return err
	}
	if permissions&discordgo.PermissionViewChannel == 0 || permissions&discordgo.PermissionSendMessages == 0 {
		return errors.New("Discord bot cannot deliver to logging destination")
	}
	return nil
}

// ticketNameSuffix keeps Discord channel names bounded and non-sensitive.
func ticketNameSuffix(ownerID string) string {
	ownerID = strings.TrimSpace(ownerID)
	if len(ownerID) > 12 {
		ownerID = ownerID[len(ownerID)-12:]
	}
	if ownerID == "" {
		return "member"
	}
	return ownerID
}

// ticketPermissionOverwrites constructs the exact private text-channel ACL.
func ticketPermissionOverwrites(guildID, ownerID, botID string, staffRoleIDs []string) []*discordgo.PermissionOverwrite {
	overwrites := []*discordgo.PermissionOverwrite{
		{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel},
		{ID: ownerID, Type: discordgo.PermissionOverwriteTypeMember, Allow: ticketChannelPermissions},
		{ID: botID, Type: discordgo.PermissionOverwriteTypeMember, Allow: ticketChannelPermissions | discordgo.PermissionManageChannels},
	}
	for _, roleID := range staffRoleIDs {
		if roleID = strings.TrimSpace(roleID); roleID != "" {
			overwrites = append(overwrites, &discordgo.PermissionOverwrite{ID: roleID, Type: discordgo.PermissionOverwriteTypeRole, Allow: ticketChannelPermissions})
		}
	}
	return overwrites
}

// validateStaffOnlyACL requires an explicit everyone denial and, when supplied,
// explicit visibility for every configured staff role.
func validateStaffOnlyACL(channel *discordgo.Channel, guildID string, staffRoleIDs []string) error {
	if channel == nil || channel.GuildID != guildID {
		return errors.New("channel is outside the configured guild")
	}
	deniedEveryone := false
	allowedRoles := map[string]bool{}
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.ID == guildID && overwrite.Type == discordgo.PermissionOverwriteTypeRole {
			if overwrite.Allow&discordgo.PermissionViewChannel != 0 {
				return errors.New("channel explicitly grants everyone visibility")
			}
			if overwrite.Deny&discordgo.PermissionViewChannel != 0 {
				deniedEveryone = true
			}
		}
		if overwrite.Type == discordgo.PermissionOverwriteTypeRole && overwrite.Allow&discordgo.PermissionViewChannel != 0 {
			allowedRoles[overwrite.ID] = true
		}
	}
	if !deniedEveryone {
		return errors.New("channel is not staff-only")
	}
	for _, roleID := range staffRoleIDs {
		if !allowedRoles[roleID] {
			return fmt.Errorf("staff role %s cannot view ticket channel", roleID)
		}
	}
	return nil
}

// validateTicketACL additionally requires explicit owner visibility.
func validateTicketACL(channel *discordgo.Channel, guildID, ownerID, botID string, staffRoleIDs []string) error {
	if err := validateStaffOnlyACL(channel, guildID, staffRoleIDs); err != nil {
		return err
	}
	ownerVisible := false
	botVisible := false
	for _, overwrite := range channel.PermissionOverwrites {
		if overwrite.ID == ownerID && overwrite.Type == discordgo.PermissionOverwriteTypeMember && overwrite.Allow&discordgo.PermissionViewChannel != 0 {
			ownerVisible = true
		}
		if overwrite.ID == botID && overwrite.Type == discordgo.PermissionOverwriteTypeMember && overwrite.Allow&discordgo.PermissionViewChannel != 0 {
			botVisible = true
		}
	}
	if !ownerVisible {
		return errors.New("ticket owner cannot view private channel")
	}
	if !botVisible {
		return errors.New("Discord bot cannot view private channel")
	}
	return nil
}

var _ tickets.DiscordClient = ticketDiscordClient{}
var _ generallogging.DeliveryClient = loggingDiscordClient{}
