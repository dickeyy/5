package discordbot

import (
	"context"
	"errors"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// AppealStaffChannelResolver returns the configured staff-only destination for an appeal event.
type AppealStaffChannelResolver interface {
	AppealStaffChannel(context.Context, string) (string, error)
}

// AppealNotificationAdapter sends appeal outbox messages through Discord without embedding staff identity.
type AppealNotificationAdapter struct {
	Session  *discordgo.Session
	Resolver AppealStaffChannelResolver
}

// SendAppealMemberNotification delivers one member-owned status update through DM.
func (a *AppealNotificationAdapter) SendAppealMemberNotification(_ context.Context, discordUserID, body string) (string, error) {
	if a == nil || a.Session == nil || strings.TrimSpace(discordUserID) == "" {
		return "", errors.New("appeal member notification adapter is not configured")
	}
	channel, err := a.Session.UserChannelCreate(discordUserID)
	if err != nil {
		return "", err
	}
	message, err := a.Session.ChannelMessageSend(channel.ID, body)
	if err != nil {
		return "", err
	}
	return message.ID, nil
}

// SendAppealStaffNotification delivers one queue entry only to a configured staff destination.
func (a *AppealNotificationAdapter) SendAppealStaffNotification(ctx context.Context, guildID, body string) (string, error) {
	if a == nil || a.Session == nil || a.Resolver == nil {
		return "", errors.New("appeal staff notification adapter is not configured")
	}
	channelID, err := a.Resolver.AppealStaffChannel(ctx, guildID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(channelID) == "" {
		return "", errors.New("appeal staff channel is unavailable")
	}
	message, err := a.Session.ChannelMessageSend(channelID, body)
	if err != nil {
		return "", err
	}
	return message.ID, nil
}
