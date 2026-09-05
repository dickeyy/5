package ui_test

import (
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/discordbot/ui"
)

// deferredResponder models Discord's first-followup aliasing of a deferred original.
type deferredResponder struct {
	ui.Responder
	completed                     bool
	originalDeleted               bool
	published                     *discordgo.Message
	editErr, followErr, deleteErr error
}

// EditOriginal consumes the deferred response so a followup can be a separate message.
func (r *deferredResponder) EditOriginal(ui.Edit) (*discordgo.Message, error) {
	if r.editErr != nil {
		return nil, r.editErr
	}
	r.completed = true
	return &discordgo.Message{ID: "original"}, nil
}

// Followup preserves the original's visibility until its defer has been completed.
func (r *deferredResponder) Followup(message ui.Message) (*discordgo.Message, error) {
	if r.followErr != nil {
		return nil, r.followErr
	}
	r.published = &discordgo.Message{ID: "result", Flags: message.WebhookParams().Flags}
	if !r.completed {
		r.published.ID = "original"
		r.published.Flags = discordgo.MessageFlagsEphemeral
	}
	return r.published, nil
}

// DeleteOriginal models the disappearing-result bug if a followup reused the original.
func (r *deferredResponder) DeleteOriginal() error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.originalDeleted = true
	if r.published != nil && r.published.ID == "original" {
		r.published = nil
	}
	return nil
}

// TestPublishSurvivesPrivateAcknowledgementCleanup reproduces the Discord lifecycle
// and ensures failures never delete an acknowledgement before a result exists.
func TestPublishSurvivesPrivateAcknowledgementCleanup(t *testing.T) {
	failure := errors.New("transport failed")
	for _, stage := range []string{"success", "edit", "followup", "cleanup"} {
		t.Run(stage, func(t *testing.T) {
			r := &deferredResponder{}
			switch stage {
			case "edit":
				r.editErr = failure
			case "followup":
				r.followErr = failure
			case "cleanup":
				r.deleteErr = failure
			}
			result, err := ui.Publish(r, ui.Content("Case created", true))
			if stage == "edit" || stage == "followup" {
				if !errors.Is(err, failure) || r.originalDeleted || r.published != nil {
					t.Fatalf("lost acknowledgement on failure: %+v, %v", r, err)
				}
				return
			}
			if err != nil || result == nil || r.published == nil || result.ID == "original" || result.Flags&discordgo.MessageFlagsEphemeral != 0 {
				t.Fatalf("result did not persist publicly: %+v, %v", r, err)
			}
			if stage == "success" && !r.originalDeleted {
				t.Fatal("private acknowledgement not cleaned up")
			}
		})
	}
}
