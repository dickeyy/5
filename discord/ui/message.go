package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	EmbedTitleLimit       = 256
	EmbedDescriptionLimit = 4096
	EmbedFieldNameLimit   = 256
	EmbedFieldValueLimit  = 1024
	EmbedFieldLimit       = 25
	EmbedFooterLimit      = 2048
	CustomIDLimit         = 100
)

const (
	ColorMain    = 0x5865F2
	ColorSuccess = 0x57F287
	ColorWarning = 0xFEE75C
	ColorError   = 0xED4245
	ColorMuted   = 0x99AAB5
)

type Message struct {
	Content         string
	Embeds          []*discordgo.MessageEmbed
	Components      []discordgo.MessageComponent
	Files           []*discordgo.File
	Ephemeral       bool
	AllowedMentions *discordgo.MessageAllowedMentions
}

type Edit struct {
	Content         *string
	Embeds          *[]*discordgo.MessageEmbed
	Components      *[]discordgo.MessageComponent
	Files           []*discordgo.File
	AllowedMentions *discordgo.MessageAllowedMentions
}

func (m Message) ResponseData() *discordgo.InteractionResponseData {
	data := &discordgo.InteractionResponseData{
		Content:         m.Content,
		Embeds:          m.Embeds,
		Components:      m.Components,
		AllowedMentions: m.AllowedMentions,
	}
	if data.AllowedMentions == nil {
		data.AllowedMentions = &discordgo.MessageAllowedMentions{}
	}
	if m.Ephemeral {
		data.Flags = discordgo.MessageFlagsEphemeral
	}
	return data
}

func (m Message) WebhookParams() *discordgo.WebhookParams {
	params := &discordgo.WebhookParams{
		Content:         m.Content,
		Embeds:          m.Embeds,
		Components:      m.Components,
		Files:           m.Files,
		AllowedMentions: m.AllowedMentions,
	}
	if params.AllowedMentions == nil {
		params.AllowedMentions = &discordgo.MessageAllowedMentions{}
	}
	if m.Ephemeral {
		params.Flags = discordgo.MessageFlagsEphemeral
	}
	return params
}

func (e Edit) WebhookEdit() *discordgo.WebhookEdit {
	edit := &discordgo.WebhookEdit{
		Content:         e.Content,
		Embeds:          e.Embeds,
		Components:      e.Components,
		Files:           e.Files,
		AllowedMentions: e.AllowedMentions,
	}
	if edit.AllowedMentions == nil {
		edit.AllowedMentions = &discordgo.MessageAllowedMentions{}
	}
	return edit
}

func EditMessage(m Message) Edit {
	content := m.Content
	embeds := m.Embeds
	components := m.Components
	return Edit{
		Content:         &content,
		Embeds:          &embeds,
		Components:      &components,
		Files:           m.Files,
		AllowedMentions: m.AllowedMentions,
	}
}

func Content(content string, ephemeral bool) Message {
	return Message{Content: content, Ephemeral: ephemeral}
}

func WithEmbeds(embeds ...*discordgo.MessageEmbed) Message {
	return Message{Embeds: embeds}
}

func EmbedMessage(embed *discordgo.MessageEmbed, ephemeral bool) Message {
	return Message{Embeds: []*discordgo.MessageEmbed{embed}, Ephemeral: ephemeral}
}

func EmbedsMessage(ephemeral bool, embeds ...*discordgo.MessageEmbed) Message {
	return Message{Embeds: embeds, Ephemeral: ephemeral}
}

func SuccessEmbed(title, description string) *discordgo.MessageEmbed {
	return NewEmbed().SetTitle(title).SetDescription(description).SetColor(ColorSuccess).SetTimestamp(time.Now()).Build()
}

func ErrorEmbed(description string) *discordgo.MessageEmbed {
	return NewEmbed().SetTitle("Error").SetDescription(description).SetColor(ColorError).SetTimestamp(time.Now()).Build()
}

func WarningEmbed(title, description string) *discordgo.MessageEmbed {
	return NewEmbed().SetTitle(title).SetDescription(description).SetColor(ColorWarning).SetTimestamp(time.Now()).Build()
}

func InfoEmbed(title, description string) *discordgo.MessageEmbed {
	return NewEmbed().SetTitle(title).SetDescription(description).SetColor(ColorMain).SetTimestamp(time.Now()).Build()
}

func TruncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

type Embed struct {
	embed *discordgo.MessageEmbed
}

func NewEmbed() *Embed {
	return &Embed{embed: &discordgo.MessageEmbed{}}
}

func NewInfoEmbed(title, description string) *Embed {
	return NewEmbed().SetTitle(title).SetDescription(description).SetColor(ColorMain)
}

func NewSuccessEmbed(title, description string) *Embed {
	return NewEmbed().SetTitle(title).SetDescription(description).SetColor(ColorSuccess)
}

func NewErrorEmbed(description string) *Embed {
	return NewEmbed().SetTitle("Error").SetDescription(description).SetColor(ColorError)
}

func (e *Embed) SetTitle(title string) *Embed {
	e.embed.Title = TruncateRunes(strings.TrimSpace(title), EmbedTitleLimit)
	return e
}

func (e *Embed) SetDescription(description string) *Embed {
	e.embed.Description = TruncateRunes(description, EmbedDescriptionLimit)
	return e
}

func (e *Embed) AddField(name string, value any, inline bool) *Embed {
	if len(e.embed.Fields) >= EmbedFieldLimit {
		return e
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "\u200b"
	}
	fieldValue := fmt.Sprint(value)
	if strings.TrimSpace(fieldValue) == "" {
		fieldValue = "\u200b"
	}
	e.embed.Fields = append(e.embed.Fields, &discordgo.MessageEmbedField{
		Name:   TruncateRunes(name, EmbedFieldNameLimit),
		Value:  TruncateRunes(fieldValue, EmbedFieldValueLimit),
		Inline: inline,
	})
	return e
}

func (e *Embed) AddFields(fields ...*discordgo.MessageEmbedField) *Embed {
	for _, field := range fields {
		if field == nil {
			continue
		}
		e.AddField(field.Name, field.Value, field.Inline)
	}
	return e
}

func (e *Embed) SetFooter(text string) *Embed {
	e.embed.Footer = &discordgo.MessageEmbedFooter{Text: TruncateRunes(text, EmbedFooterLimit)}
	return e
}

func (e *Embed) SetAuthor(name, iconURL string) *Embed {
	e.embed.Author = &discordgo.MessageEmbedAuthor{
		Name:    TruncateRunes(strings.TrimSpace(name), EmbedTitleLimit),
		IconURL: strings.TrimSpace(iconURL),
	}
	return e
}

func (e *Embed) SetThumbnail(url string) *Embed {
	e.embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: strings.TrimSpace(url)}
	return e
}

func (e *Embed) SetTimestamp(t time.Time) *Embed {
	if t.IsZero() {
		t = time.Now()
	}
	e.embed.Timestamp = t.UTC().Format(time.RFC3339)
	return e
}

func (e *Embed) SetColor(color int) *Embed {
	e.embed.Color = color
	return e
}

func (e *Embed) SetNamedColor(name string) *Embed {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "success", "green":
		return e.SetColor(ColorSuccess)
	case "warning", "yellow":
		return e.SetColor(ColorWarning)
	case "error", "red", "danger":
		return e.SetColor(ColorError)
	case "muted", "gray", "grey":
		return e.SetColor(ColorMuted)
	default:
		return e.SetColor(ColorMain)
	}
}

func Field(name string, value any, inline bool) *discordgo.MessageEmbedField {
	return &discordgo.MessageEmbedField{
		Name:   fmt.Sprint(name),
		Value:  fmt.Sprint(value),
		Inline: inline,
	}
}

func (e *Embed) Build() *discordgo.MessageEmbed {
	return e.embed
}
