package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bwmarrin/discordgo"
	actionmods "github.com/quackdiscord/bot/app/actions"
	runtimeDiscord "github.com/quackdiscord/bot/discord"
)

type DiscordActionClient interface {
	SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error)
}

type DiscordActionError = actionmods.DiscordError

type DiscordRuntimeActionClient struct{}

func NewDiscordActionClient() *DiscordRuntimeActionClient {
	return &DiscordRuntimeActionClient{}
}

func (c *DiscordRuntimeActionClient) SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session := runtimeDiscord.Session
	if session == nil {
		return nil, actionmods.DiscordError{Code: "discord_session_unavailable", Message: "discord session is unavailable", Retryable: true}
	}

	channel, err := session.UserChannelCreate(discordUserID)
	if err != nil {
		return nil, classifyDiscordError("send_dm_channel", err)
	}
	sent, err := session.ChannelMessageSend(channel.ID, message)
	if err != nil {
		return nil, classifyDiscordError("send_dm_message", err)
	}

	response := map[string]any{"channel_id": channel.ID}
	if sent != nil {
		response["message_id"] = sent.ID
	}
	return response, nil
}

func classifyDiscordError(code string, err error) error {
	if err == nil {
		return nil
	}

	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil {
		status := restErr.Response.StatusCode
		retryable := status == http.StatusTooManyRequests || status >= 500
		return actionmods.DiscordError{
			Code:      fmt.Sprintf("%s_%d", code, status),
			Message:   err.Error(),
			Retryable: retryable,
		}
	}

	return actionmods.DiscordError{
		Code:      code,
		Message:   err.Error(),
		Retryable: true,
	}
}
