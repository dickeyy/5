package discordbot

import (
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

const discordUserGuildsURL = "https://discord.com/api/v10/users/@me/guilds"

// Bot owns the Discord session and adapts Discord guild and messaging operations to core ports.
type Bot struct {
	Session    *discordgo.Session
	HTTPClient *http.Client
	connected  atomic.Bool
}

// New constructs new with required dependencies explicit so callers control lifecycle and substitution.
func New(token string) (*Bot, error) {
	session, err := discordgo.New(token)
	if err != nil {
		return nil, err
	}
	// Runtime adds only the intents required by currently enabled optional
	// modules before opening the gateway.
	session.Identify.Intents = discordgo.IntentGuilds
	session.StateEnabled = true
	session.State.MaxMessageCount = 5000
	bot := &Bot{Session: session, HTTPClient: http.DefaultClient}
	session.AddHandler(bot.gatewayReady)
	session.AddHandler(bot.gatewayResumed)
	session.AddHandler(bot.gatewayConnected)
	session.AddHandler(bot.gatewayDisconnected)
	return bot, nil
}

// gatewayReady marks the authenticated initial gateway session ready.
func (b *Bot) gatewayReady(_ *discordgo.Session, _ *discordgo.Ready) { b.connected.Store(true) }

// gatewayResumed marks a successfully resumed gateway session ready.
func (b *Bot) gatewayResumed(_ *discordgo.Session, _ *discordgo.Resumed) { b.connected.Store(true) }

// gatewayConnected marks DiscordGo's post-handshake synthetic connect event ready.
func (b *Bot) gatewayConnected(_ *discordgo.Session, _ *discordgo.Connect) { b.connected.Store(true) }

// gatewayDisconnected immediately removes Discord from process readiness while reconnecting.
func (b *Bot) gatewayDisconnected(_ *discordgo.Session, _ *discordgo.Disconnect) {
	b.connected.Store(false)
}

// Open opens and verifies open so startup fails before serving traffic when the dependency is unavailable.
func (b *Bot) Open() error {
	if b == nil || b.Session == nil {
		return errors.New("discord session is not configured")
	}
	if err := b.Session.Open(); err != nil {
		return err
	}
	b.connected.Store(true)
	return nil
}

// Close releases resources owned by bot and is safe to use during reverse-order shutdown.
func (b *Bot) Close() error {
	if b == nil || b.Session == nil {
		return nil
	}
	if !b.connected.Swap(false) {
		return nil
	}
	return b.Session.Close()
}

// Status reports whether the adapter's external dependency is currently ready for health checks.
func (b *Bot) Status() (bool, string, int64) {
	if b == nil || !b.connected.Load() || b.Session == nil || b.Session.State == nil || b.Session.State.User == nil {
		return false, "", 0
	}
	return true, b.Session.State.User.Username, b.Session.HeartbeatLatency().Milliseconds()
}
