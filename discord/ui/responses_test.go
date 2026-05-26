package ui_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/discord/ui"
)

func TestPublicAndEphemeralResponses(t *testing.T) {
	public := ui.Public(ui.Content("visible", false))
	if public.Type != discordgo.InteractionResponseChannelMessageWithSource {
		t.Fatalf("unexpected public response type: %v", public.Type)
	}
	if public.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public response without ephemeral flag")
	}

	ephemeral := ui.Ephemeral(ui.Content("private", false))
	if ephemeral.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response flag")
	}
}

func TestDeferredResponses(t *testing.T) {
	public := ui.DeferPublic()
	if public.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("unexpected defer public type: %v", public.Type)
	}
	if public.Data != nil && public.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public defer without ephemeral flag")
	}

	ephemeral := ui.DeferEphemeral()
	if ephemeral.Type != discordgo.InteractionResponseDeferredChannelMessageWithSource {
		t.Fatalf("unexpected defer ephemeral type: %v", ephemeral.Type)
	}
	if ephemeral.Data == nil || ephemeral.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral defer flag")
	}
}

func TestEmbedTruncatesByRuneLimit(t *testing.T) {
	embed := ui.NewEmbed().
		SetTitle(strings.Repeat("t", ui.EmbedTitleLimit+10)).
		SetDescription(strings.Repeat("d", ui.EmbedDescriptionLimit+10)).
		AddField(strings.Repeat("n", ui.EmbedFieldNameLimit+10), strings.Repeat("v", ui.EmbedFieldValueLimit+10), false).
		SetFooter(strings.Repeat("f", ui.EmbedFooterLimit+10)).
		Build()

	if len([]rune(embed.Title)) != ui.EmbedTitleLimit {
		t.Fatalf("title was not truncated")
	}
	if len([]rune(embed.Description)) != ui.EmbedDescriptionLimit {
		t.Fatalf("description was not truncated")
	}
	if len([]rune(embed.Fields[0].Name)) != ui.EmbedFieldNameLimit {
		t.Fatalf("field name was not truncated")
	}
	if len([]rune(embed.Fields[0].Value)) != ui.EmbedFieldValueLimit {
		t.Fatalf("field value was not truncated")
	}
	if len([]rune(embed.Footer.Text)) != ui.EmbedFooterLimit {
		t.Fatalf("footer was not truncated")
	}
}

func TestCustomIDCodec(t *testing.T) {
	encoded, err := ui.EncodeCustomID(ui.CustomID{
		Namespace: "case",
		Action:    "next",
		Version:   "v1",
		Payload:   "target=123",
	})
	if err != nil {
		t.Fatalf("encode custom id: %v", err)
	}
	if encoded != "case:next:v1:target=123" {
		t.Fatalf("unexpected custom id: %q", encoded)
	}

	decoded, err := ui.DecodeCustomID(encoded)
	if err != nil {
		t.Fatalf("decode custom id: %v", err)
	}
	if decoded.Namespace != "case" || decoded.Action != "next" || decoded.Version != "v1" || decoded.Payload != "target=123" {
		t.Fatalf("unexpected decoded custom id: %+v", decoded)
	}
}

func TestCustomIDRejectsInvalidAndTooLongValues(t *testing.T) {
	if _, err := ui.DecodeCustomID("case:missing"); !errors.Is(err, ui.ErrCustomIDInvalid) {
		t.Fatalf("expected invalid custom id error, got %v", err)
	}
	_, err := ui.EncodeCustomID(ui.CustomID{
		Namespace: "case",
		Action:    "next",
		Version:   "v1",
		Payload:   strings.Repeat("x", ui.CustomIDLimit),
	})
	if !errors.Is(err, ui.ErrCustomIDTooLong) {
		t.Fatalf("expected too long custom id error, got %v", err)
	}
}
