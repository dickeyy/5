package commands

import (
	"context"
	"encoding/json"
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
const messageCaseCommandName = "Create moderation case"

// CaseCommandSpec binds the /case definition to its interaction handler for explicit runtime registration.
func CaseCommandSpec() CommandSpec {
	return CommandSpec{
		Definition: CaseCommandDefinition(),
		Handler:    HandleCaseInteraction,
	}
}

// MessageCaseCommandSpec starts the same evidence-backed flow from a live Discord message.
func MessageCaseCommandSpec() CommandSpec {
	permissions := int64(discordgo.PermissionModerateMembers)
	dm := false
	return CommandSpec{Definition: &discordgo.ApplicationCommand{Type: discordgo.MessageApplicationCommand, Name: messageCaseCommandName, DefaultMemberPermissions: &permissions, DMPermission: &dm}, Handler: HandleMessageCaseInteraction}
}

// HandleMessageCaseInteraction derives the target from the selected live message and applies the sole active policy, or directs staff to explicit template selection.
func HandleMessageCaseInteraction(ctx ui.Context) ui.HandlerResult {
	interaction := ctx.Interaction
	if interaction == nil || interaction.GuildID == "" {
		return ui.Immediate(ui.Error("Select a server message to create a case."))
	}
	data := interaction.ApplicationCommandData()
	message := data.Resolved.Messages[data.TargetID]
	if message == nil || message.Author == nil {
		return ui.Immediate(ui.Error("The selected message is unavailable."))
	}
	guildContext, err := resolveInteractionGuildContext(ctx.Context, ctx.Services, interaction)
	if err != nil {
		return ui.Immediate(ui.Error(caseCommandErrorMessage(err)))
	}
	templates, err := ctx.Services.Templates.ListActive(ctx.Context, guildContext)
	if err != nil || len(templates) == 0 {
		return ui.Immediate(ui.Error("No active case template is available."))
	}
	if len(templates) > 1 {
		options := make([]discordgo.SelectMenuOption, 0, len(templates))
		for _, template := range templates {
			options = append(options, discordgo.SelectMenuOption{Label: templateAutocompleteLabel(template), Value: template.ID})
			if len(options) == 25 {
				break
			}
		}
		customID, encodeErr := ui.EncodeCustomID(ui.CustomID{Namespace: "case", Action: "message_template", Version: "v1", Payload: strings.Join([]string{message.Author.ID, message.ChannelID, message.ID}, "|")})
		if encodeErr != nil {
			return ui.Immediate(ui.Error("Use `/case add` to select a template for this message."))
		}
		selectMenu := discordgo.SelectMenu{CustomID: customID, Placeholder: "Choose an active case template", MinValues: intPointer(1), MaxValues: 1, Options: options}
		return ui.Immediate(ui.Ephemeral(ui.Message{Content: "Choose the template that matches this message.", Components: []discordgo.MessageComponent{ui.Row(selectMenu)}, Ephemeral: true}))
	}
	template := templates[0]
	if len(template.ContextFields) > 1 || (len(template.ContextFields) == 1 && template.ContextFields[0].FieldType != model.ContextFieldMessageLink) {
		return ui.Immediate(ui.Error("Use `/case add` to complete this template's visible context."))
	}
	link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", interaction.GuildID, message.ChannelID, message.ID)
	values := []quack.CaseContextValueInput{}
	if len(template.ContextFields) == 1 {
		raw, _ := json.Marshal(link)
		values = append(values, quack.CaseContextValueInput{Key: template.ContextFields[0].Key, Value: raw})
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		created, createErr := ctx.Services.Cases.Create(taskCtx, guildContext, quack.CaseInput{TemplateID: template.ID, TargetDiscordUserID: message.Author.ID, Source: model.CaseSourceDiscord, ContextChannelDiscordID: message.ChannelID, ContextMessageDiscordID: message.ID, ContextValues: values, EvidenceLinks: []string{link}, IdempotencyKey: interaction.ID})
		if createErr != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(createErr)))
			return editErr
		}
		message, followErr := responder.Followup(views.CaseCreatedMessage(views.CaseCreated{Case: created, Template: &template}))
		if followErr == nil {
			_ = responder.DeleteOriginal()
			if message != nil {
				updatePublicCaseResult(responder, ctx.Services, created, message.ID, &template)
			}
		}
		return followErr
	})
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
		return handleCaseStaffSubcommand(ctx, data)
	}

	if err := validateCaseInteraction(ctx.Context, ctx.Services, interaction, add); err != nil {
		return ui.Immediate(ui.Error(caseCommandErrorMessage(err)))
	}
	if contextOption := add.GetOption("context"); contextOption == nil || strings.TrimSpace(contextOption.StringValue()) == "" {
		guildContext, resolveErr := resolveInteractionGuildContext(ctx.Context, ctx.Services, interaction)
		if resolveErr != nil {
			return ui.Immediate(ui.Error(caseCommandErrorMessage(resolveErr)))
		}
		_, template, templateErr := resolveTemplate(ctx.Context, ctx.Services, guildContext, optionStringValue(add.GetOption("template")))
		if templateErr != nil {
			return ui.Immediate(ui.Error(caseCommandErrorMessage(templateErr)))
		}
		if template != nil && len(template.ContextFields) > 0 {
			modal, modalErr := contextModal(interaction, template, optionStringValue(add.GetOption("user")), optionStringValue(add.GetOption("message_link")))
			if modalErr != nil {
				return ui.Immediate(ui.Error(modalErr.Error()))
			}
			return ui.Immediate(modal)
		}
	}

	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		result, err := createCaseFromInteraction(taskCtx, ctx.Services, interaction, add)
		if err != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(err)))
			if editErr != nil {
				return editErr
			}
			return nil
		}

		message, err := responder.Followup(views.CaseCreatedMessage(views.CaseCreated{
			Case:     result.Case,
			Template: result.Template,
		}))
		if err == nil {
			_ = responder.DeleteOriginal()
			if message != nil {
				updatePublicCaseResult(responder, ctx.Services, result.Case, message.ID, result.Template)
			}
		}
		return err
	})
}

// handleCaseStaffSubcommand provides privacy-safe Discord case browsing and recovery controls.
func handleCaseStaffSubcommand(ctx ui.Context, data discordgo.ApplicationCommandInteractionData) ui.HandlerResult {
	var selected *discordgo.ApplicationCommandInteractionDataOption
	for _, option := range data.Options {
		if option != nil {
			selected = option
			break
		}
	}
	if selected == nil {
		return ui.Immediate(ui.Error("Choose a case operation."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		guildContext, err := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
		if err != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(err)))
			return editErr
		}
		var response ui.Message
		switch selected.Name {
		case "view":
			detail, getErr := ctx.Services.Cases.Get(taskCtx, guildContext, optionStringValue(selected.GetOption("case")))
			err = getErr
			if detail != nil {
				response = views.CaseDetailMessage(detail)
			}
		case "list":
			list, listErr := ctx.Services.Cases.List(taskCtx, guildContext, quack.CaseListInput{Limit: "10"})
			err = listErr
			if list != nil {
				response = views.CaseListMessage(list, 1, "")
			}
		case "user":
			targetID := optionStringValue(selected.GetOption("user"))
			profile, profileErr := ctx.Services.Cases.UserHistory(taskCtx, guildContext, targetID, quack.CaseListInput{Limit: "10"})
			err = profileErr
			if profile != nil {
				response = views.CaseListMessage(&quack.CaseListResponse{Cases: profile.Cases, Total: profile.Total, Limit: profile.Limit, Offset: profile.Offset}, 1, targetID)
			}
		case "failures":
			failed, failedErr := ctx.Services.Actions.ListFailures(taskCtx, guildContext, 10, 0)
			err = failedErr
			if failed != nil {
				response = views.FailedActionMessage(failed, 1)
			}
		case "retry":
			_, err = ctx.Services.Actions.Retry(taskCtx, guildContext, optionStringValue(selected.GetOption("execution")))
			response = ui.EmbedMessage(ui.SuccessEmbed("Action retry queued", "The same configured action will be attempted after current permission and hierarchy checks."), true)
		case "dismiss":
			_, err = ctx.Services.Actions.Dismiss(taskCtx, guildContext, optionStringValue(selected.GetOption("execution")))
			response = ui.EmbedMessage(ui.SuccessEmbed("Action failure dismissed", "Attempt history remains visible on the case."), true)
		case "void":
			if confirm := selected.GetOption("confirm"); confirm == nil || !confirm.BoolValue() {
				err = quack.ErrCaseValidation
			} else {
				_, err = ctx.Services.Cases.Void(taskCtx, guildContext, optionStringValue(selected.GetOption("case")), optionStringValue(selected.GetOption("reason")), nil)
				response = ui.EmbedMessage(ui.SuccessEmbed("Case voided", "The correction remains visible in history."), true)
			}
		case "reverse":
			if confirm := selected.GetOption("confirm"); confirm == nil || !confirm.BoolValue() {
				err = quack.ErrCaseValidation
			} else {
				_, err = ctx.Services.Actions.Reverse(taskCtx, guildContext, optionStringValue(selected.GetOption("case")), optionStringValue(selected.GetOption("execution")), model.ActionType(optionStringValue(selected.GetOption("action"))))
				response = ui.EmbedMessage(ui.SuccessEmbed("Reversal queued", "The original action and reversal remain visible in history."), true)
			}
		default:
			err = quack.ErrCaseValidation
		}
		if err != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(err)))
			return editErr
		}
		_, editErr := responder.EditOriginal(ui.EditMessage(response))
		return editErr
	})
}

func intPointer(value int) *int { return &value }

func updatePublicCaseResult(responder ui.Responder, services *quack.Services, created *quack.CaseResponse, messageID string, template *quack.TemplateResponse) {
	if services == nil || services.Store == nil || responder == nil || created == nil || created.ID == "" || messageID == "" {
		return
	}
	refresh := func() (*quack.CaseResponse, bool) {
		actions, err := services.Store.ListCaseActionExecutions(context.Background(), created.ID)
		if err != nil {
			return nil, false
		}
		copy := *created
		byID := make(map[string]model.CaseActionExecution, len(actions))
		for _, action := range actions {
			byID[action.ID] = action
		}
		terminal := true
		for index := range copy.Actions {
			if current, ok := byID[copy.Actions[index].ID]; ok {
				copy.Actions[index].Status = current.Status
			}
			switch copy.Actions[index].Status {
			case model.ActionExecutionPending, model.ActionExecutionRunning, model.ActionExecutionRetrying:
				terminal = false
			}
		}
		return &copy, terminal
	}
	if current, terminal := refresh(); current != nil && terminal {
		_, _ = responder.EditFollowup(messageID, ui.EditMessage(views.CaseCreatedMessage(views.CaseCreated{Case: current, Template: template})))
		return
	}
	go func() {
		for attempt := 0; attempt < 300; attempt++ {
			time.Sleep(100 * time.Millisecond)
			current, terminal := refresh()
			if current != nil && terminal {
				_, _ = responder.EditFollowup(messageID, ui.EditMessage(views.CaseCreatedMessage(views.CaseCreated{Case: current, Template: template})))
				return
			}
		}
	}()
}

// validateCaseInteraction checks case interaction before state is read or changed.
func validateCaseInteraction(ctx context.Context, services *quack.Services, interaction *discordgo.InteractionCreate, add *discordgo.ApplicationCommandInteractionDataOption) error {
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
	guildContext, err := resolveInteractionGuildContext(ctx, services, interaction)
	if err != nil {
		return err
	}
	return services.Guilds.Authorize(ctx, guildContext, model.PermissionActionCaseCreate, model.AuditSourceDiscord)
}

// CaseCommandDefinition returns a fresh /case definition so Discord-side mutation cannot alter registry state.
func CaseCommandDefinition() *discordgo.ApplicationCommand {
	defaultPermissions := int64(discordgo.PermissionModerateMembers)
	dmPermission := false
	add := &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: "add", Description: "Create a moderation case from a template.", Options: []*discordgo.ApplicationCommandOption{{Type: discordgo.ApplicationCommandOptionString, Name: "template", Description: "Case template to apply.", Required: true, Autocomplete: true}, {Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "User to moderate.", Required: true}, {Type: discordgo.ApplicationCommandOptionString, Name: "context", Description: "Visible context values as a JSON object."}, {Type: discordgo.ApplicationCommandOptionString, Name: "message_link", Description: "Discord message link to capture as evidence."}}}
	options := append([]*discordgo.ApplicationCommandOption{add}, caseStaffCommandOptions()...)
	return &discordgo.ApplicationCommand{
		Name:                     caseCommandName,
		Description:              "Create and manage moderation cases.",
		DefaultMemberPermissions: &defaultPermissions,
		DMPermission:             &dmPermission,
		Options:                  options,
	}
}

// caseStaffCommandOptions defines privacy-safe browsing and explicit recovery controls.
func caseStaffCommandOptions() []*discordgo.ApplicationCommandOption {
	stringOption := func(name, description string, required bool) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionString, Name: name, Description: description, Required: required}
	}
	sub := func(name, description string, options ...*discordgo.ApplicationCommandOption) *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionSubCommand, Name: name, Description: description, Options: options}
	}
	confirm := func() *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionBoolean, Name: "confirm", Description: "Confirm this irreversible control.", Required: true}
	}
	return []*discordgo.ApplicationCommandOption{sub("view", "View authorized case detail.", stringOption("case", "Case number or ID.", true)), sub("list", "List recent guild cases."), sub("user", "View a member's case history.", &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionUser, Name: "user", Description: "Member to review.", Required: true}), sub("failures", "Review failed Discord actions."), sub("retry", "Retry the same failed action.", stringOption("execution", "Failed execution ID.", true)), sub("dismiss", "Dismiss a failure from active review.", stringOption("execution", "Failed execution ID.", true)), sub("void", "Void an incorrect case.", stringOption("case", "Case number or ID.", true), stringOption("reason", "Required correction reason.", true), confirm()), sub("reverse", "Remove a timeout or unban.", stringOption("case", "Case ID.", true), stringOption("execution", "Original execution ID.", true), &discordgo.ApplicationCommandOption{Type: discordgo.ApplicationCommandOptionString, Name: "action", Description: "Reversal action.", Required: true, Choices: []*discordgo.ApplicationCommandOptionChoice{{Name: "Remove timeout", Value: string(model.ActionRemoveTimeout)}, {Name: "Unban", Value: string(model.ActionUnbanUser)}}}, confirm())}
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
	if err := services.Guilds.Authorize(ctx, guildContext, model.PermissionActionCaseCreate, model.AuditSourceDiscord); err != nil {
		return nil, err
	}

	templateID, template, err := resolveTemplate(ctx, services, guildContext, templateOption.StringValue())
	if err != nil {
		return nil, err
	}

	contextValues := contextValuesFromOption(add.GetOption("context"), template)
	contextValues = mergeMessageLinkContext(contextValues, add.GetOption("message_link"), template)
	created, err := services.Cases.Create(ctx, guildContext, quack.CaseInput{
		TemplateID:              templateID,
		TargetDiscordUserID:     optionStringValue(userOption),
		Source:                  model.CaseSourceDiscord,
		ContextChannelDiscordID: interaction.ChannelID,
		ContextValues:           contextValues, EvidenceLinks: evidenceLinksFromOption(add.GetOption("message_link")), IdempotencyKey: interaction.ID,
	})
	if err != nil {
		return nil, err
	}

	return &caseCommandCreateResult{Case: created, Template: template}, nil
}

// mergeMessageLinkContext binds a pasted link to the first Discord-message-link definition when not already supplied.
func mergeMessageLinkContext(values []quack.CaseContextValueInput, option *discordgo.ApplicationCommandInteractionDataOption, template *quack.TemplateResponse) []quack.CaseContextValueInput {
	if option == nil || template == nil || strings.TrimSpace(option.StringValue()) == "" {
		return values
	}
	existing := map[string]struct{}{}
	for _, value := range values {
		existing[value.Key] = struct{}{}
	}
	for _, field := range template.ContextFields {
		if field.FieldType == model.ContextFieldMessageLink {
			if _, ok := existing[field.Key]; !ok {
				raw, _ := json.Marshal(strings.TrimSpace(option.StringValue()))
				values = append(values, quack.CaseContextValueInput{Key: field.Key, Value: raw})
			}
			break
		}
	}
	return values
}

// contextValuesFromOption decodes visible template context without accepting a reason override.
func contextValuesFromOption(option *discordgo.ApplicationCommandInteractionDataOption, template *quack.TemplateResponse) []quack.CaseContextValueInput {
	if option == nil || template == nil {
		return nil
	}
	var values map[string]json.RawMessage
	if json.Unmarshal([]byte(option.StringValue()), &values) != nil {
		return []quack.CaseContextValueInput{{Key: "__invalid__", Value: json.RawMessage(`null`)}}
	}
	out := make([]quack.CaseContextValueInput, 0, len(values))
	for key, value := range values {
		out = append(out, quack.CaseContextValueInput{Key: key, Value: value})
	}
	return out
}

// evidenceLinksFromOption forwards pasted links into the shared capture service.
func evidenceLinksFromOption(option *discordgo.ApplicationCommandInteractionDataOption) []string {
	if option == nil || strings.TrimSpace(option.StringValue()) == "" {
		return nil
	}
	return []string{strings.TrimSpace(option.StringValue())}
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

	templates, err := services.Templates.ListActive(ctx, guildContext)
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
	if err != nil || services.Guilds.Authorize(ctx, guildContext, model.PermissionActionCaseCreate, model.AuditSourceDiscord) != nil {
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
	templates, err := services.Templates.ListActive(ctx, guildContext)
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
	case errors.Is(err, quack.ErrCasePermissionDenied), errors.Is(err, quack.ErrAuthorizationDenied):
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
