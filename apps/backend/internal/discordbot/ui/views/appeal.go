package views

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/discordbot/ui"
	"github.com/quackdiscord/bot/internal/quack"
)

// AppealStaffMessage renders a staff-only appeal timeline and explicit reversal offers.
func AppealStaffMessage(appeal *quack.AppealResponse) ui.Message {
	if appeal == nil {
		return ui.EmbedMessage(ui.ErrorEmbed("Appeal not found."), true)
	}
	embed := ui.NewInfoEmbed("Appeal Review", "").
		AddField("Case", appeal.CaseID, true).
		AddField("Member", fmt.Sprintf("<@%s>", appeal.TargetDiscordUserID), true).
		AddField("Status", appeal.Status, true).
		SetFooter("Appeal ID: " + appeal.ID)
	for _, event := range appeal.Events {
		actor := event.ActorType
		if event.ActorDiscordUserID != "" {
			actor += " <@" + event.ActorDiscordUserID + ">"
		}
		embed.AddField(string(event.Type), actor+": "+event.Body, false)
	}
	message := ui.EmbedMessage(embed.Build(), true)
	for _, offer := range appeal.ReversalOffers {
		customID, err := ui.EncodeCustomID(ui.CustomID{Namespace: "appeal", Action: "reverse", Version: "v1", Payload: appeal.ID + "," + offer.OriginalExecutionID + "," + string(offer.ActionType)})
		if err != nil {
			continue
		}
		message.Components = append(message.Components, ui.Row(ui.Button(customID, "Confirm "+string(offer.ActionType), discordgo.DangerButton, false)))
	}
	return message
}

// AppealEntryMessage creates a secure dashboard link for an eligible case notification.
func AppealEntryMessage(baseURL, guildID, caseID string) (ui.Message, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ui.Message{}, fmt.Errorf("secure dashboard base URL is required")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/guilds/" + url.PathEscape(guildID) + "/cases/" + url.PathEscape(caseID) + "/appeal"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return ui.Message{Content: "This case is eligible for appeal.", Components: []discordgo.MessageComponent{ui.Row(ui.LinkButton(parsed.String(), "Open appeal", false))}}, nil
}
