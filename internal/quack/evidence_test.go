package quack_test

import (
	"context"
	"errors"
	"testing"

	"github.com/quackdiscord/bot/internal/quack"
)

type unavailableEvidenceClient struct{ err error }

func (f unavailableEvidenceClient) FetchMessageEvidence(context.Context, quack.DiscordMessageReference) (*quack.DiscordMessageSnapshot, error) {
	return nil, f.err
}
func (unavailableEvidenceClient) PreserveEvidenceAttachment(context.Context, string, string, quack.DiscordAttachmentSnapshot) (*quack.PreservedDiscordAttachment, error) {
	return nil, nil
}
func (unavailableEvidenceClient) EnsureEvidenceChannel(context.Context, string, string) (string, error) {
	return "", nil
}

func TestParseDiscordMessageLinkRejectsLookalikesAndCrossGuildCapture(t *testing.T) {
	valid := "https://discord.com/channels/111111111111111111/222222222222222222/333333333333333333"
	ref, err := quack.ParseDiscordMessageLink(valid)
	if err != nil || ref.MessageID != "333333333333333333" {
		t.Fatalf("valid link: %+v err=%v", ref, err)
	}
	for _, invalid := range []string{"https://discord.example/channels/111111111111111111/222222222222222222/333333333333333333", "https://discord.com/channels/@me/2/3", "javascript:alert(1)"} {
		if _, err := quack.ParseDiscordMessageLink(invalid); !errors.Is(err, quack.ErrEvidenceValidation) {
			t.Fatalf("accepted invalid link %q: %v", invalid, err)
		}
	}
	service := quack.NewEvidenceService(unavailableEvidenceClient{})
	if _, err := service.Capture(context.Background(), "999999999999999999", "target", "", []string{valid}, false); !errors.Is(err, quack.ErrEvidenceValidation) {
		t.Fatalf("cross-guild capture accepted: %v", err)
	}
}

func TestUnavailableEvidenceRequiresOtherVisibleContext(t *testing.T) {
	link := "https://discord.com/channels/111111111111111111/222222222222222222/333333333333333333"
	service := quack.NewEvidenceService(unavailableEvidenceClient{err: &quack.EvidenceUnavailableError{Outcome: "deleted", Message: "message deleted"}})
	if _, err := service.Capture(context.Background(), "111111111111111111", "target", "", []string{link}, false); err == nil {
		t.Fatal("deleted message continued without visible fallback context")
	}
	captured, err := service.Capture(context.Background(), "111111111111111111", "target", "", []string{link}, true)
	if err != nil || len(captured.Snapshots) != 1 || captured.Snapshots[0].CaptureOutcome != "deleted" || captured.Snapshots[0].MessageCreatedAt.IsZero() {
		t.Fatalf("partial capture: %+v err=%v", captured, err)
	}
}

func FuzzParseDiscordMessageLink(f *testing.F) {
	f.Add("https://discord.com/channels/111111111111111111/222222222222222222/333333333333333333")
	f.Add("not-a-url")
	f.Fuzz(func(t *testing.T, value string) {
		ref, err := quack.ParseDiscordMessageLink(value)
		if err == nil && (ref.GuildID == "" || ref.ChannelID == "" || ref.MessageID == "") {
			t.Fatalf("successful parse returned empty identity: %+v", ref)
		}
	})
}
