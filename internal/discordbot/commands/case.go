package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/discordbot/ui"
	"github.com/quackdiscord/bot/internal/discordbot/ui/views"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
	"github.com/rs/zerolog/log"
)

const caseCommandName = "case"

// CaseCommandSpec binds the /case definition to its interaction handler for explicit runtime registration.
func CaseCommandSpec() CommandSpec {
	return CommandSpec{
		Definition: CaseCommandDefinition(),
		Handler:    HandleCaseInteraction,
	}
}

// HandleCaseInteraction handles case interaction and translates it into the package's application or response contract.
func HandleCaseInteraction(ctx ui.Context) ui.HandlerResult {
	interaction := ctx.Interaction
	if interaction == nil || interaction.Interaction == nil {
		return ui.HandlerResult{}
	}
	if interaction.Type == discordgo.InteractionApplicationCommandAutocomplete {
		return ui.Immediate(handleTemplateAutocomplete(ctx.Context, ctx.Services, interaction))
	}

	data := interaction.ApplicationCommandData()
	add := data.GetOption("add")
	if add == nil {
		return ui.Immediate(ui.Error("Use `/case add` to create a case from a template."))
	}

	if err := validateCaseInteraction(ctx.Services, interaction, add); err != nil {
		return ui.Immediate(ui.Error(caseCommandErrorMessage(err)))
	}

	return ui.Async(ui.DeferPublic(), func(taskCtx context.Context, responder ui.Responder) error {
		result, err := createCaseFromInteraction(taskCtx, ctx.Services, interaction, add)
		if err != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(err)))
			if editErr != nil {
				return editErr
			}
			return nil
		}

		_, err = responder.EditOriginal(ui.EditMessage(views.CaseCreatedMessage(views.CaseCreated{
			Case:     result.Case,
			Template: result.Template,
		})))
		return err
	})
}

// validateCaseInteraction checks case interaction before state is read or changed.
func validateCaseInteraction(services *quack.Services, interaction *discordgo.InteractionCreate, add *discordgo.ApplicationCommandInteractionDataOption) error {
	if services == nil || services.Guilds == nil || services.Cases == nil {
		return errors.New("case command services are not configured")
	}
	if interaction.GuildID == "" {
		return errors.New("case commands must be used in a server")
	}

	templateOption := add.GetOption("template")
	userOption := add.GetOption("user")
	if templateOption == nil || userOption == nil {
		return quack.ErrCaseValidation
	}

	_, _, permissionBits := interactionMemberFields(interaction)
	if permissionBits&uint64(discordgo.PermissionAdministrator) == 0 &&
		permissionBits&uint64(discordgo.PermissionModerateMembers) == 0 {
		return quack.ErrCasePermissionDenied
	}
	return nil
}

// CaseCommandDefinition returns a fresh /case definition so Discord-side mutation cannot alter registry state.
func CaseCommandDefinition() *discordgo.ApplicationCommand {
	defaultPermissions := int64(discordgo.PermissionModerateMembers)
	dmPermission := false

	return &discordgo.ApplicationCommand{
		Name:                     caseCommandName,
		Description:              "Create and manage moderation cases.",
		DefaultMemberPermissions: &defaultPermissions,
		DMPermission:             &dmPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "add",
				Description: "Create a moderation case from a template.",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionString,
						Name:         "template",
						Description:  "Case template to apply.",
						Required:     true,
						Autocomplete: true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionUser,
						Name:        "user",
						Description: "User to moderate.",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "reason",
						Description: "Optional reason override.",
						Required:    false,
					},
				},
			},
		},
	}
}

// CommandDefinition returns a fresh compatibility command definition derived from the explicit case specification.
func CommandDefinition() *discordgo.ApplicationCommand {
	return CaseCommandDefinition()
}

// caseCommandCreateResult captures the outcome of case command create result for the caller.
type caseCommandCreateResult struct {
	Case     *quack.CaseResponse
	Template *quack.TemplateResponse
}

// createCaseFromInteraction creates case from interaction while preserving validation, authorization, and persistence invariants.
func createCaseFromInteraction(ctx context.Context, services *quack.Services, interaction *discordgo.InteractionCreate, add *discordgo.ApplicationCommandInteractionDataOption) (*caseCommandCreateResult, error) {
	if services == nil || services.Guilds == nil || services.Cases == nil {
		return nil, errors.New("case command services are not configured")
	}
	if interaction.GuildID == "" {
		return nil, errors.New("case commands must be used in a server")
	}

	templateOption := add.GetOption("template")
	userOption := add.GetOption("user")
	if templateOption == nil || userOption == nil {
		return nil, quack.ErrCaseValidation
	}

	guildContext, err := resolveInteractionGuildContext(ctx, services, interaction)
	if err != nil {
		return nil, err
	}
	if !guildContext.Can(model.PermissionActionCaseCreate) {
		return nil, quack.ErrCasePermissionDenied
	}

	templateID, template, err := resolveTemplate(ctx, services, guildContext, templateOption.StringValue())
	if err != nil {
		return nil, err
	}

	reason := ""
	if reasonOption := add.GetOption("reason"); reasonOption != nil {
		reason = reasonOption.StringValue()
	}

	created, err := services.Cases.Create(ctx, guildContext, quack.CaseInput{
		TemplateID:              templateID,
		TargetDiscordUserID:     optionStringValue(userOption),
		ReasonOverride:          reason,
		Source:                  model.CaseSourceDiscordCommand,
		ContextChannelDiscordID: interaction.ChannelID,
	})
	if err != nil {
		return nil, err
	}

	return &caseCommandCreateResult{Case: created, Template: template}, nil
}

// resolveInteractionGuildContext resolves interaction guild context from authoritative request and repository data.
func resolveInteractionGuildContext(ctx context.Context, services *quack.Services, interaction *discordgo.InteractionCreate) (*quack.GuildStaffContext, error) {
	userID, displayName, permissionBits := interactionMemberFields(interaction)
	return services.Guilds.ResolveDiscordStaffContext(ctx, quack.DiscordStaffContextInput{
		DiscordGuildID: interaction.GuildID,
		DiscordUserID:  userID,
		DisplayName:    displayName,
		PermissionBits: permissionBits,
		LastActiveAt:   time.Now().UTC(),
	})
}

// resolveTemplate resolves template from authoritative request and repository data.
func resolveTemplate(ctx context.Context, services *quack.Services, guildContext *quack.GuildStaffContext, templateInput string) (string, *quack.TemplateResponse, error) {
	value := strings.TrimSpace(templateInput)
	if value == "" {
		return "", nil, quack.ErrCaseValidation
	}

	templates, err := services.Templates.List(ctx, guildContext)
	if err != nil {
		return "", nil, err
	}
	for _, template := range templates {
		if template.ID == value || strings.EqualFold(template.Slug, value) {
			matched := template
			return template.ID, &matched, nil
		}
	}

	return value, nil, nil
}

// handleTemplateAutocomplete handles template autocomplete and translates it into the package's application or response contract.
func handleTemplateAutocomplete(ctx context.Context, services *quack.Services, interaction *discordgo.InteractionCreate) *discordgo.InteractionResponse {
	guildContext, err := resolveInteractionGuildContext(ctx, services, interaction)
	if err != nil || !guildContext.Can(model.PermissionActionCaseCreate) {
		return autocompleteResponse(nil)
	}

	data := interaction.ApplicationCommandData()
	add := data.GetOption("add")
	if add == nil {
		return autocompleteResponse(nil)
	}
	templateOption := add.GetOption("template")
	if templateOption == nil {
		return autocompleteResponse(nil)
	}

	query := strings.ToLower(strings.TrimSpace(templateOption.StringValue()))
	templates, err := services.Templates.List(ctx, guildContext)
	if err != nil {
		log.Error().Err(err).Msg("failed to list templates for case autocomplete")
		return autocompleteResponse(nil)
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, 25)
	for _, template := range templates {
		search := strings.ToLower(template.Slug + " " + template.Name + " " + template.Description)
		if query != "" && !strings.Contains(search, query) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  templateAutocompleteLabel(template),
			Value: template.ID,
		})
		if len(choices) == 25 {
			break
		}
	}

	return autocompleteResponse(choices)
}

// interactionMemberFields encapsulates the interaction member fields rule so callers share one consistent package implementation.
func interactionMemberFields(interaction *discordgo.InteractionCreate) (string, string, uint64) {
	if interaction == nil || interaction.Member == nil {
		return "", "", 0
	}

	member := interaction.Member
	userID := ""
	username := ""
	if member.User != nil {
		userID = member.User.ID
		username = member.User.Username
	}

	displayName := strings.TrimSpace(member.Nick)
	if displayName == "" && member.User != nil {
		displayName = strings.TrimSpace(member.User.GlobalName)
	}
	if displayName == "" {
		displayName = strings.TrimSpace(username)
	}

	return userID, displayName, uint64(member.Permissions)
}

// optionStringValue encapsulates the option string value rule so callers share one consistent package implementation.
func optionStringValue(option *discordgo.ApplicationCommandInteractionDataOption) string {
	if option == nil || option.Value == nil {
		return ""
	}
	switch value := option.Value.(type) {
	case string:
		return value
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

// templateAutocompleteLabel encapsulates the template autocomplete label rule so callers share one consistent package implementation.
func templateAutocompleteLabel(template quack.TemplateResponse) string {
	name := strings.TrimSpace(template.Name)
	if name == "" {
		name = strings.TrimSpace(template.Slug)
	}
	description := strings.TrimSpace(template.Description)
	if description != "" {
		name = fmt.Sprintf("%s - %s", name, description)
	}
	return truncateDiscordChoiceName(name)
}

// truncateDiscordChoiceName encapsulates the truncate discord choice name rule so callers share one consistent package implementation.
func truncateDiscordChoiceName(value string) string {
	runes := []rune(value)
	if len(runes) <= 100 {
		return value
	}
	return string(runes[:100])
}

// caseCommandErrorMessage converts case command error message into its transport presentation without leaking transport types into the core.
func caseCommandErrorMessage(err error) string {
	switch {
	case errors.Is(err, quack.ErrCasePermissionDenied):
		return "You do not have permission to create that case."
	case errors.Is(err, quack.ErrCaseTemplateNotAvailable):
		return "That case template is not available."
	case errors.Is(err, quack.ErrCaseValidation):
		return "That case request is invalid."
	case errors.Is(err, quack.ErrBotNotInGuild):
		return "Quack is not active in this server."
	default:
		log.Error().Err(err).Msg("case command failed")
		return "Quack could not create that case."
	}
}

// autocompleteResponse converts autocomplete response into its transport presentation without leaking transport types into the core.
func autocompleteResponse(choices []*discordgo.ApplicationCommandOptionChoice) *discordgo.InteractionResponse {
	return ui.Autocomplete(choices)
}
