package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/discordbot/ui"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// CaseCreated groups the case created state used to keep this package's responsibilities explicit.
type CaseCreated struct {
	Case     *quack.CaseResponse
	Template *quack.TemplateResponse
}

// CaseCreatedMessage converts case created message into its transport presentation without leaking transport types into the core.
func CaseCreatedMessage(result CaseCreated) ui.Message {
	return ui.EmbedMessage(CaseCreatedEmbed(result), false)
}

// CaseCreatedEmbed converts case created embed into its transport presentation without leaking transport types into the core.
func CaseCreatedEmbed(result CaseCreated) *discordgo.MessageEmbed {
	if result.Case == nil {
		return ui.SuccessEmbed("Case Created", "Created case.")
	}

	created := result.Case
	embed := ui.NewSuccessEmbed(fmt.Sprintf("Case #%d Created", created.CaseNumber), "").
		AddField("Target", fmt.Sprintf("<@%s>", created.TargetDiscordUserID), true).
		AddField("Moderator", fmt.Sprintf("<@%s>", created.ModeratorDiscordUserID), true).
		SetFooter(fmt.Sprintf("Case ID: %s", created.ID)).
		SetTimestamp(time.Now())

	templateName := caseTemplateDisplayName(result.Template)
	if templateName != "" {
		embed.AddField("Template", templateName, false)
	}

	if created.SelectedLevel != nil {
		levelName := strings.TrimSpace(created.SelectedLevel.Name)
		if levelName == "" {
			levelName = fmt.Sprintf("Level %d", created.SelectedLevel.Position)
		}
		embed.AddField("Level", levelName, true)
		if created.SelectedLevel.MatchedCaseCount > 0 {
			embed.AddField("Matching Cases", created.SelectedLevel.MatchedCaseCount, true)
		}
	}

	embed.AddField("Queued Actions", fmt.Sprintf("%d%s", len(created.Actions), actionSummary(created.Actions)), false)
	return embed.Build()
}

// FormatCaseCreated encapsulates the format case created rule so callers share one consistent package implementation.
func FormatCaseCreated(result CaseCreated) string {
	if result.Case == nil {
		return "Created case."
	}

	created := result.Case
	lines := []string{
		fmt.Sprintf("Case #%d created", created.CaseNumber),
		fmt.Sprintf("Target: <@%s>", created.TargetDiscordUserID),
		fmt.Sprintf("Moderator: <@%s>", created.ModeratorDiscordUserID),
	}

	templateName := caseTemplateDisplayName(result.Template)
	if templateName != "" {
		lines = append(lines, "Template: "+templateName)
	}

	if created.SelectedLevel != nil {
		levelName := strings.TrimSpace(created.SelectedLevel.Name)
		if levelName == "" {
			levelName = fmt.Sprintf("Level %d", created.SelectedLevel.Position)
		}
		lines = append(lines, fmt.Sprintf("Level: %s", levelName))
		if created.SelectedLevel.MatchedCaseCount > 0 {
			lines = append(lines, fmt.Sprintf("Matching cases: %d", created.SelectedLevel.MatchedCaseCount))
		}
	}

	lines = append(lines, fmt.Sprintf("Queued actions: %d%s", len(created.Actions), actionSummary(created.Actions)))
	return strings.Join(lines, "\n")
}

// caseTemplateDisplayName encapsulates the case template display name rule so callers share one consistent package implementation.
func caseTemplateDisplayName(template *quack.TemplateResponse) string {
	if template == nil {
		return ""
	}
	name := strings.TrimSpace(template.Name)
	slug := strings.TrimSpace(template.Slug)
	switch {
	case name != "" && slug != "" && !strings.EqualFold(name, slug):
		return fmt.Sprintf("%s (`%s`)", name, slug)
	case name != "":
		return name
	default:
		return slug
	}
}

// actionSummary encapsulates the action summary rule so callers share one consistent package implementation.
func actionSummary(actions []quack.CaseActionResponse) string {
	if len(actions) == 0 {
		return " (none)"
	}

	counts := map[model.ActionType]int{}
	order := make([]model.ActionType, 0, len(actions))
	for _, action := range actions {
		if counts[action.ActionType] == 0 {
			order = append(order, action.ActionType)
		}
		counts[action.ActionType]++
	}

	parts := make([]string, 0, len(order))
	for _, actionType := range order {
		count := counts[actionType]
		if count == 1 {
			parts = append(parts, string(actionType))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s x%d", actionType, count))
	}
	return ": " + strings.Join(parts, ", ")
}
