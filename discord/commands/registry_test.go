package commands

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/discord/ui"
)

func TestRegistryRejectsDuplicateCommandNames(t *testing.T) {
	registry := NewRegistry()
	spec := CommandSpec{
		Definition: &discordgo.ApplicationCommand{Name: "case", Description: "one"},
		Handler:    noopHandler,
	}
	if err := registry.Register(spec); err != nil {
		t.Fatalf("register command: %v", err)
	}

	if err := registry.Register(spec); err == nil {
		t.Fatalf("expected duplicate command registration to fail")
	}
}

func TestDefaultRegistryIncludesCaseCommand(t *testing.T) {
	spec, ok := defaultRegistry.Lookup("case")
	if !ok {
		t.Fatalf("expected case command to be registered")
	}
	if spec.Definition == nil || spec.Handler == nil {
		t.Fatalf("expected complete case command spec, got %+v", spec)
	}
}

func noopHandler(ctx ui.Context) ui.HandlerResult {
	_ = context.Background()
	return ui.HandlerResult{}
}
