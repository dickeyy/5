package ui

import (
	"strings"

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

func (e *Embed) SetTitle(title string) *Embed {
	e.embed.Title = TruncateRunes(strings.TrimSpace(title), EmbedTitleLimit)
	return e
}

func (e *Embed) SetDescription(description string) *Embed {
	e.embed.Description = TruncateRunes(description, EmbedDescriptionLimit)
	return e
}

func (e *Embed) AddField(name, value string, inline bool) *Embed {
	if len(e.embed.Fields) >= EmbedFieldLimit {
		return e
	}
	e.embed.Fields = append(e.embed.Fields, &discordgo.MessageEmbedField{
		Name:   TruncateRunes(strings.TrimSpace(name), EmbedFieldNameLimit),
		Value:  TruncateRunes(value, EmbedFieldValueLimit),
		Inline: inline,
	})
	return e
}

func (e *Embed) SetFooter(text string) *Embed {
	e.embed.Footer = &discordgo.MessageEmbedFooter{Text: TruncateRunes(text, EmbedFooterLimit)}
	return e
}

func (e *Embed) SetColor(color int) *Embed {
	e.embed.Color = color
	return e
}

func (e *Embed) Build() *discordgo.MessageEmbed {
	return e.embed
}
