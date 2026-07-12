package discordbot

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestBotStatusTracksGatewayDisconnectAndResume(t *testing.T) {
	bot, err := New("token")
	if err != nil {
		t.Fatalf("new bot: %v", err)
	}
	bot.Session.State.User = &discordgo.User{ID: "bot-1", Username: "quack"}
	bot.gatewayReady(bot.Session, &discordgo.Ready{})
	if connected, _, _ := bot.Status(); !connected {
		t.Fatal("expected ready gateway to be connected")
	}
	bot.gatewayDisconnected(bot.Session, &discordgo.Disconnect{})
	if connected, _, _ := bot.Status(); connected {
		t.Fatal("expected disconnected gateway to fail readiness")
	}
	bot.gatewayResumed(bot.Session, &discordgo.Resumed{})
	if connected, _, _ := bot.Status(); !connected {
		t.Fatal("expected resumed gateway to restore readiness")
	}
}
