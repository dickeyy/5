package discordbot

import (
	"context"
	"errors"

	"log/slog"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
)

// GuildLifecycleHandler translates Discord guild and channel events into idempotent core lifecycle operations.
type GuildLifecycleHandler struct {
	Guilds   *quack.GuildService
	Evidence *quack.EvidenceService
}

// RegisterGuildLifecycle installs lifecycle handlers before the gateway opens so initial GuildCreate events cannot be missed.
func RegisterGuildLifecycle(session *discordgo.Session, services *quack.Services) error {
	if session == nil {
		return errors.New("discord session is not configured")
	}
	if services == nil || services.Guilds == nil {
		return errors.New("guild service is not configured")
	}
	handler := &GuildLifecycleHandler{Guilds: services.Guilds, Evidence: services.Evidence}
	session.AddHandler(handler.HandleGuildCreate)
	session.AddHandler(handler.HandleGuildUpdate)
	session.AddHandler(handler.HandleGuildDelete)
	session.AddHandler(handler.HandleChannelDelete)
	return nil
}

// HandleGuildCreate installs a new guild or reactivates known history and repairs channel references from the complete create payload.
func (h *GuildLifecycleHandler) HandleGuildCreate(_ *discordgo.Session, event *discordgo.GuildCreate) {
	if h == nil || h.Guilds == nil || event == nil || event.Guild == nil || event.Unavailable {
		return
	}
	input := guildLifecycleInput(event.Guild, channelIDs(event.Channels))
	result, err := h.Guilds.BootstrapDiscordGuild(context.Background(), input)
	if err != nil {
		slog.Error("Failed to bootstrap Discord guild", "error", err, "guild_id", event.ID)
		return
	}
	if h.Evidence != nil && result != nil {
		if _, err := h.Evidence.EnsureGuildEvidenceChannel(context.Background(), result.Guild, result.Settings); err != nil {
			slog.Error("Failed to ensure managed evidence channel", "error", err, "guild_id", event.ID)
		}
	}
}

// HandleGuildUpdate refreshes authoritative name, icon, owner, and active state without treating a partial payload as channel deletion.
func (h *GuildLifecycleHandler) HandleGuildUpdate(_ *discordgo.Session, event *discordgo.GuildUpdate) {
	if h == nil || h.Guilds == nil || event == nil || event.Guild == nil || event.Unavailable {
		return
	}
	result, err := h.Guilds.BootstrapDiscordGuild(context.Background(), guildLifecycleInput(event.Guild, nil))
	if err != nil {
		slog.Error("Failed to refresh Discord guild", "error", err, "guild_id", event.ID)
		return
	}
	if h.Evidence != nil && result != nil {
		if _, err := h.Evidence.EnsureGuildEvidenceChannel(context.Background(), result.Guild, result.Settings); err != nil {
			slog.Error("Detected managed evidence channel drift", "error", err, "guild_id", event.ID)
		}
	}
}

// HandleGuildDelete marks a true bot removal inactive while ignoring Discord's temporary unavailable signal.
func (h *GuildLifecycleHandler) HandleGuildDelete(_ *discordgo.Session, event *discordgo.GuildDelete) {
	if h == nil || h.Guilds == nil || event == nil || event.Guild == nil || event.Unavailable {
		return
	}
	if _, err := h.Guilds.DeactivateDiscordGuild(context.Background(), event.ID); err != nil {
		slog.Error("Failed to deactivate departed Discord guild", "error", err, "guild_id", event.ID)
	}
}

// HandleChannelDelete clears configured references to a channel Discord confirms was deleted.
func (h *GuildLifecycleHandler) HandleChannelDelete(_ *discordgo.Session, event *discordgo.ChannelDelete) {
	if h == nil || h.Guilds == nil || event == nil || event.Channel == nil || event.GuildID == "" {
		return
	}
	if _, err := h.Guilds.ClearDeletedChannel(context.Background(), event.GuildID, event.ID); err != nil {
		slog.Error("Failed to clear deleted Discord channel reference", "error", err, "guild_id", event.GuildID, "channel_id", event.ID)
	}
	if h.Evidence != nil {
		if _, err := h.Evidence.RepairDiscordGuildEvidenceChannel(context.Background(), event.GuildID); err != nil {
			slog.Error("Failed to repair managed evidence channel after deletion", "error", err, "guild_id", event.GuildID)
		}
	}
}

// guildLifecycleInput maps Discord gateway metadata into the transport-neutral lifecycle contract.
func guildLifecycleInput(guild *discordgo.Guild, knownChannelIDs []string) quack.DiscordGuildLifecycleInput {
	return quack.DiscordGuildLifecycleInput{
		DiscordGuildID: guild.ID, Name: guild.Name, Icon: guild.Icon,
		OwnerDiscordUserID: guild.OwnerID, KnownChannelDiscordIDs: knownChannelIDs,
	}
}

// channelIDs returns the complete channel identity inventory supplied by a GuildCreate event.
func channelIDs(channels []*discordgo.Channel) []string {
	if channels == nil {
		return nil
	}
	ids := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel != nil && channel.ID != "" {
			ids = append(ids, channel.ID)
		}
	}
	return ids
}
