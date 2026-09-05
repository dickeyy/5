package moduleintegration

import (
	"context"
	"encoding/json"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/modules"
	"github.com/quackdiscord/bot/internal/modules/generallogging"
	"github.com/quackdiscord/bot/internal/modules/honeypot"
)

// onGuildMemberAdd queues configured member-join logging.
func (r *Runtime) onGuildMemberAdd(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
	r.memberEvent(event.Member, generallogging.MemberJoin)
}

// onGuildMemberRemove queues configured member-leave logging.
func (r *Runtime) onGuildMemberRemove(_ *discordgo.Session, event *discordgo.GuildMemberRemove) {
	r.memberEvent(event.Member, generallogging.MemberLeave)
}

// memberEvent maps one Discord member lifecycle event into module identity.
func (r *Runtime) memberEvent(member *discordgo.Member, eventType generallogging.EventType) {
	if member == nil {
		return
	}
	guildID, ok := r.internalGuildID(member.GuildID)
	if !ok {
		return
	}
	actorID := ""
	if member.User != nil {
		actorID = member.User.ID
	}
	r.submit(generallogging.Event{GuildID: guildID, Type: eventType, ActorDiscordUserID: actorID})
}

// onGuildBanAdd queues configured ban logging.
func (r *Runtime) onGuildBanAdd(_ *discordgo.Session, event *discordgo.GuildBanAdd) {
	r.banEvent(event.GuildID, event.User, generallogging.DiscordBan)
}

// onGuildBanRemove queues configured unban logging.
func (r *Runtime) onGuildBanRemove(_ *discordgo.Session, event *discordgo.GuildBanRemove) {
	r.banEvent(event.GuildID, event.User, generallogging.DiscordUnban)
}

// banEvent maps one Discord ban lifecycle event into module identity.
func (r *Runtime) banEvent(discordGuildID string, user *discordgo.User, eventType generallogging.EventType) {
	guildID, ok := r.internalGuildID(discordGuildID)
	if !ok {
		return
	}
	actorID := ""
	if user != nil {
		actorID = user.ID
	}
	r.submit(generallogging.Event{GuildID: guildID, Type: eventType, ActorDiscordUserID: actorID})
}

// onGuildUpdate queues non-content guild metadata changes.
func (r *Runtime) onGuildUpdate(_ *discordgo.Session, event *discordgo.GuildUpdate) {
	if event == nil || event.Guild == nil {
		return
	}
	guildID, ok := r.internalGuildID(event.ID)
	if ok {
		r.submit(generallogging.Event{GuildID: guildID, Type: generallogging.GuildChange, Metadata: map[string]string{"name": event.Name}})
	}
}

// onGuildDelete disables the departed guild's honeypot while retaining its
// configuration for an explicit repair after a future rejoin.
func (r *Runtime) onGuildDelete(_ *discordgo.Session, event *discordgo.GuildDelete) {
	if r == nil || r.registry == nil || r.HoneypotDiscord == nil || event == nil || event.Guild == nil || event.Unavailable {
		return
	}
	ctx := context.Background()
	guildID, err := r.resolver.internalIDAny(ctx, event.ID)
	if err != nil {
		return
	}
	configuration, err := r.registry.Configuration(ctx, guildID, modules.Honeypots)
	if err != nil || configuration == nil || !configuration.Enabled {
		return
	}
	var settings honeypot.Settings
	if json.Unmarshal([]byte(configuration.ConfigJSON), &settings) != nil || settings.ChannelDiscordID == "" {
		return
	}
	_ = r.HoneypotDiscord.HandleDeletedChannel(ctx, guildID, settings.ChannelDiscordID)
}

// onChannelCreate queues configured channel creation logging.
func (r *Runtime) onChannelCreate(_ *discordgo.Session, event *discordgo.ChannelCreate) {
	if event != nil {
		r.channelEvent(event.Channel, "created")
	}
}

// onChannelUpdate queues configured channel update logging.
func (r *Runtime) onChannelUpdate(_ *discordgo.Session, event *discordgo.ChannelUpdate) {
	if event != nil {
		r.channelEvent(event.Channel, "updated")
	}
}

// channelEvent maps one Discord channel lifecycle event into module identity.
func (r *Runtime) channelEvent(channel *discordgo.Channel, operation string) {
	if channel == nil || channel.GuildID == "" {
		return
	}
	guildID, ok := r.internalGuildID(channel.GuildID)
	if ok {
		r.submit(generallogging.Event{GuildID: guildID, ChannelDiscordID: channel.ID, Type: generallogging.ChannelChange, Metadata: map[string]string{"operation": operation, "name": channel.Name}})
	}
}

// onChannelDelete queues logging and repairs every module reference.
func (r *Runtime) onChannelDelete(_ *discordgo.Session, event *discordgo.ChannelDelete) {
	if event == nil || event.Channel == nil {
		return
	}
	r.channelEvent(event.Channel, "deleted")
	guildID, ok := r.internalGuildID(event.GuildID)
	if !ok {
		return
	}
	ctx := context.Background()
	if r.TicketDiscord != nil {
		_ = r.TicketDiscord.HandleDeletedEntryChannel(ctx, guildID, event.ID)
	}
	if r.HoneypotDiscord != nil {
		_ = r.HoneypotDiscord.HandleDeletedChannel(ctx, guildID, event.ID)
	}
	var ticket struct {
		ID      string
		GuildID string
	}
	result := r.db.WithContext(ctx).Table("tickets").Select("id, guild_id").Where("guild_id = ? AND thread_discord_channel_id = ?", guildID, event.ID).Limit(1).Find(&ticket)
	if r.TicketDiscord != nil && result.Error == nil && result.RowsAffected == 1 {
		_ = r.TicketDiscord.HandleDeletedChannel(ctx, ticket.GuildID, ticket.ID, event.ID)
	}
	if r.Logging != nil {
		_, _, _ = r.Logging.RepairDeletedChannel(ctx, generallogging.Actor{GuildID: guildID, DiscordUserID: "quack-system", CanManage: true}, event.ID)
	}
}
