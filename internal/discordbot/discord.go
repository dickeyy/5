package discordbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/actionmods"
)

const discordUserGuildsURL = "https://discord.com/api/v10/users/@me/guilds"

// Bot owns the Discord session and adapts Discord guild and messaging operations to core ports.
type Bot struct {
	Session    *discordgo.Session
	HTTPClient *http.Client
}

// New constructs new with required dependencies explicit so callers control lifecycle and substitution.
func New(token string) (*Bot, error) {
	session, err := discordgo.New(token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.Intent(3276543)
	session.StateEnabled = true
	session.State.MaxMessageCount = 5000
	return &Bot{Session: session, HTTPClient: http.DefaultClient}, nil
}

// Open opens and verifies open so startup fails before serving traffic when the dependency is unavailable.
func (b *Bot) Open() error {
	if b == nil || b.Session == nil {
		return errors.New("discord session is not configured")
	}
	return b.Session.Open()
}

// Close releases resources owned by bot and is safe to use during reverse-order shutdown.
func (b *Bot) Close() error {
	if b == nil || b.Session == nil {
		return nil
	}
	return b.Session.Close()
}

// Status reports whether the adapter's external dependency is currently ready for health checks.
func (b *Bot) Status() (bool, string, int64) {
	if b == nil || b.Session == nil || b.Session.State == nil || b.Session.State.User == nil {
		return false, "", 0
	}
	return true, b.Session.State.User.Username, b.Session.HeartbeatLatency().Milliseconds()
}

// UserGuilds encapsulates the user guilds rule so callers share one consistent package implementation.
func (b *Bot) UserGuilds(ctx context.Context, accessToken string) ([]quack.DiscordUserGuild, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("missing discord access token")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserGuildsURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	client := b.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("discord user guilds failed with status %d", response.StatusCode)
	}
	var guilds []quack.DiscordUserGuild
	if err := json.NewDecoder(response.Body).Decode(&guilds); err != nil {
		return nil, err
	}
	return guilds, nil
}

// BotGuild encapsulates the bot guild rule so callers share one consistent package implementation.
func (b *Bot) BotGuild(ctx context.Context, guildID string) (*quack.DiscordBotGuild, error) {
	if b == nil || b.Session == nil {
		return nil, quack.ErrBotNotInGuild
	}
	if b.Session.State != nil {
		if guild, err := b.Session.State.Guild(guildID); err == nil && guild != nil {
			return botGuild(guild), nil
		}
	}
	guild, err := b.Session.Guild(guildID)
	if err != nil {
		return nil, quack.ErrBotNotInGuild
	}
	return botGuild(guild), nil
}

// BotGuilds encapsulates the bot guilds rule so callers share one consistent package implementation.
func (b *Bot) BotGuilds(context.Context) ([]quack.DiscordBotGuild, error) {
	if b == nil || b.Session == nil || b.Session.State == nil {
		return []quack.DiscordBotGuild{}, nil
	}
	b.Session.State.RLock()
	defer b.Session.State.RUnlock()
	guilds := make([]quack.DiscordBotGuild, 0, len(b.Session.State.Guilds))
	for _, guild := range b.Session.State.Guilds {
		if guild != nil {
			guilds = append(guilds, *botGuild(guild))
		}
	}
	return guilds, nil
}

// botGuild encapsulates the bot guild rule so callers share one consistent package implementation.
func botGuild(guild *discordgo.Guild) *quack.DiscordBotGuild {
	return &quack.DiscordBotGuild{ID: guild.ID, Name: guild.Name, Icon: guild.Icon, OwnerID: guild.OwnerID}
}

// SendDM sends dm through the configured external gateway.
func (b *Bot) SendDM(ctx context.Context, userID, message string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.Session == nil {
		return nil, actionmods.DiscordError{Code: "discord_session_unavailable", Message: "discord session is unavailable", Retryable: true}
	}
	channel, err := b.Session.UserChannelCreate(userID)
	if err != nil {
		return nil, classifyDiscordError("send_dm_channel", err)
	}
	sent, err := b.Session.ChannelMessageSend(channel.ID, message)
	if err != nil {
		return nil, classifyDiscordError("send_dm_message", err)
	}
	result := map[string]any{"channel_id": channel.ID}
	if sent != nil {
		result["message_id"] = sent.ID
	}
	return result, nil
}

// classifyDiscordError encapsulates the classify discord error rule so callers share one consistent package implementation.
func classifyDiscordError(code string, err error) error {
	var restError *discordgo.RESTError
	if errors.As(err, &restError) && restError.Response != nil {
		status := restError.Response.StatusCode
		return actionmods.DiscordError{Code: fmt.Sprintf("%s_%d", code, status), Message: err.Error(), Retryable: status == http.StatusTooManyRequests || status >= 500}
	}
	return actionmods.DiscordError{Code: code, Message: err.Error(), Retryable: true}
}
