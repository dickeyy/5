package discordbot

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
)

// SendAuditMirror sends one core audit event to its configured staff-only channel.
// It does not share formatting, queues, or state with optional general logging.
func (b *Bot) SendAuditMirror(ctx context.Context, message quack.AuditMirrorMessage) error {
	if b == nil || b.Session == nil {
		return errors.New("Discord audit mirror adapter is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	fields := []*discordgo.MessageEmbedField{
		{Name: "Result", Value: string(message.Result), Inline: true},
		{Name: "Resource", Value: fmt.Sprintf("%s · `%s`", message.ResourceType, message.ResourceID), Inline: true},
	}
	if message.ActorDiscordUserID != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Actor", Value: "<@" + message.ActorDiscordUserID + ">", Inline: true})
	}
	if message.FailureReason != "" {
		fields = append(fields, &discordgo.MessageEmbedField{Name: "Failure", Value: truncateAuditMirrorText(message.FailureReason, 256)})
	}
	trace := strings.TrimSpace(message.CorrelationID)
	if trace == "" {
		trace = strings.TrimSpace(message.RequestID)
	}
	embed := &discordgo.MessageEmbed{Title: truncateAuditMirrorText(message.Action, 256), Description: "Quack moderation audit event", Fields: fields, Color: auditMirrorColor(string(message.Result)), Timestamp: message.OccurredAt.UTC().Format(time.RFC3339), Footer: &discordgo.MessageEmbedFooter{Text: "Audit " + message.AuditEntryID + " · Trace " + trace}}
	_, err := b.Session.ChannelMessageSendComplex(message.ChannelDiscordID, &discordgo.MessageSend{Embed: embed, AllowedMentions: &discordgo.MessageAllowedMentions{}})
	if err == nil {
		return nil
	}
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil && (restErr.Response.StatusCode == http.StatusForbidden || restErr.Response.StatusCode == http.StatusNotFound) {
		return fmt.Errorf("%w: Discord rejected configured channel", quack.ErrAuditMirrorChannelUnavailable)
	}
	return errors.New("Discord audit mirror delivery failed")
}

func auditMirrorColor(result string) int {
	switch result {
	case "success":
		return 0x57F287
	case "denied":
		return 0xFEE75C
	default:
		return 0xED4245
	}
}

func truncateAuditMirrorText(value string, limit int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit])
}
