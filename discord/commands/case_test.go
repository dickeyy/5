package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/internal/testutil"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

type fakeDiscordClient struct {
	botGuild *app.DiscordBotGuild
}

func (f fakeDiscordClient) UserGuilds(ctx context.Context, accessToken string) ([]app.DiscordUserGuild, error) {
	return nil, nil
}

func (f fakeDiscordClient) BotGuilds(ctx context.Context) ([]app.DiscordBotGuild, error) {
	if f.botGuild == nil {
		return nil, nil
	}
	return []app.DiscordBotGuild{*f.botGuild}, nil
}

func (f fakeDiscordClient) BotGuild(ctx context.Context, discordGuildID string) (*app.DiscordBotGuild, error) {
	return f.botGuild, nil
}

func TestCommandDefinitionDefinesCaseAdd(t *testing.T) {
	command := CommandDefinition()
	if command.Name != "case" || command.DMPermission == nil || *command.DMPermission {
		t.Fatalf("unexpected command definition: %+v", command)
	}
	if command.DefaultMemberPermissions == nil || *command.DefaultMemberPermissions != int64(discordgo.PermissionModerateMembers) {
		t.Fatalf("expected moderate members default permission, got %+v", command.DefaultMemberPermissions)
	}
	if len(command.Options) != 1 || command.Options[0].Name != "add" {
		t.Fatalf("expected add subcommand, got %+v", command.Options)
	}

	add := command.Options[0]
	if len(add.Options) != 3 {
		t.Fatalf("expected template/user/reason options, got %+v", add.Options)
	}
	if !add.Options[0].Autocomplete {
		t.Fatalf("expected template option to support autocomplete")
	}
}

func TestHandleCaseInteractionCreatesCase(t *testing.T) {
	ctx := context.Background()
	store, services, templateID := newCaseCommandHarness(t)

	response := HandleCaseInteraction(ctx, services, nil, caseAddInteraction(templateID, "target-1", "manual reason", uint64(discordgo.PermissionModerateMembers)))
	if response == nil || response.Data == nil {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public success response, got flags %d", response.Data.Flags)
	}
	for _, want := range []string{
		"Case #1 created",
		"Target: <@target-1>",
		"Moderator: <@mod-1>",
		"Template: Spam",
		"Level: Default",
		"Matching cases: 1",
		"Queued actions: 1",
		"send_dm",
	} {
		if !strings.Contains(response.Data.Content, want) {
			t.Fatalf("expected response to contain %q, got %q", want, response.Data.Content)
		}
	}

	cases, err := store.ListCases(ctx, storeGuildID(t, store, "guild-1"))
	if err != nil {
		t.Fatalf("list cases: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("expected one case, got %+v", cases)
	}
	if cases[0].Source != structs.CaseSourceDiscordCommand || cases[0].TargetDiscordUserID != "target-1" {
		t.Fatalf("unexpected case: %+v", cases[0])
	}
	if cases[0].Reason != "manual reason" {
		t.Fatalf("expected reason override, got %q", cases[0].Reason)
	}
}

func TestHandleCaseInteractionDeniesMissingPermission(t *testing.T) {
	_, services, templateID := newCaseCommandHarness(t)

	response := HandleCaseInteraction(context.Background(), services, nil, caseAddInteraction(templateID, "target-1", "", 0))
	if response == nil || response.Data == nil || !strings.Contains(response.Data.Content, "do not have permission") {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral error response, got flags %d", response.Data.Flags)
	}
}

func TestHandleTemplateAutocompleteReturnsUsableTemplates(t *testing.T) {
	_, services, templateID := newCaseCommandHarness(t)

	response := HandleCaseInteraction(context.Background(), services, nil, templateAutocompleteInteraction("repeated", uint64(discordgo.PermissionModerateMembers)))
	if response == nil || response.Data == nil || len(response.Data.Choices) != 1 {
		t.Fatalf("unexpected autocomplete response: %+v", response)
	}
	if response.Data.Choices[0].Value != templateID {
		t.Fatalf("expected template id choice, got %+v", response.Data.Choices[0])
	}
	if response.Data.Choices[0].Name != "Spam - Unwanted repeated messages" {
		t.Fatalf("expected name and description choice label, got %q", response.Data.Choices[0].Name)
	}
}

func TestHandleTemplateAutocompleteFiltersDisabledTemplates(t *testing.T) {
	_, services, _ := newCaseCommandHarness(t)
	ctx := context.Background()
	guildContext := caseCommandGuildContext(t, services)
	enabled := false
	createCaseCommandTemplate(t, services, guildContext, app.TemplateInput{
		Slug:           "ghost",
		Name:           "Ghost",
		Description:    "Hidden moderation workflow",
		ReasonTemplate: "Hidden",
		Enabled:        &enabled,
		Levels: []app.TemplateLevelInput{
			{
				Name:      "Default",
				Position:  1,
				IsDefault: true,
			},
		},
	})

	response := HandleCaseInteraction(ctx, services, nil, templateAutocompleteInteraction("hidden", uint64(discordgo.PermissionModerateMembers)))
	if response == nil || response.Data == nil {
		t.Fatalf("unexpected autocomplete response: %+v", response)
	}
	if len(response.Data.Choices) != 0 {
		t.Fatalf("expected disabled template to be filtered, got %+v", response.Data.Choices)
	}
}

func TestTemplateAutocompleteLabelTruncatesToDiscordLimit(t *testing.T) {
	_, services, _ := newCaseCommandHarness(t)
	ctx := context.Background()
	guildContext := caseCommandGuildContext(t, services)
	longTemplate := createCaseCommandTemplate(t, services, guildContext, app.TemplateInput{
		Slug:           "longdesc",
		Name:           "Long Description",
		Description:    strings.Repeat("description ", 20),
		ReasonTemplate: "Long description",
		Levels: []app.TemplateLevelInput{
			{
				Name:      "Default",
				Position:  1,
				IsDefault: true,
			},
		},
	})

	response := HandleCaseInteraction(ctx, services, nil, templateAutocompleteInteraction("longdesc", uint64(discordgo.PermissionModerateMembers)))
	if response == nil || response.Data == nil || len(response.Data.Choices) != 1 {
		t.Fatalf("unexpected autocomplete response: %+v", response)
	}
	if response.Data.Choices[0].Value != longTemplate.ID {
		t.Fatalf("expected long template id choice, got %+v", response.Data.Choices[0])
	}
	if len([]rune(response.Data.Choices[0].Name)) != 100 {
		t.Fatalf("expected label length 100, got %d: %q", len([]rune(response.Data.Choices[0].Name)), response.Data.Choices[0].Name)
	}
}

func newCaseCommandHarness(t *testing.T) (*storage.Store, *app.Services, string) {
	t.Helper()

	ctx := context.Background()
	store := testutil.NewSQLiteStore(t)
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	services := app.NewWithDiscordClient(store, fakeDiscordClient{
		botGuild: &app.DiscordBotGuild{ID: "guild-1", Name: "Guild", OwnerID: "owner-1"},
	})
	guildContext, err := services.Guilds.ResolveDiscordStaffContext(ctx, app.DiscordStaffContextInput{
		DiscordGuildID: "guild-1",
		DiscordUserID:  "owner-1",
		DisplayName:    "Owner",
	})
	if err != nil {
		t.Fatalf("resolve guild context: %v", err)
	}

	created := createCaseCommandTemplate(t, services, guildContext, app.TemplateInput{
		Slug:           "spam",
		Name:           "Spam",
		Description:    "Unwanted repeated messages",
		ReasonTemplate: "Spam",
		Levels: []app.TemplateLevelInput{
			{
				Name:       "Default",
				Position:   1,
				IsDefault:  true,
				NotifyUser: true,
			},
		},
	})

	return store, services, created.ID
}

func createCaseCommandTemplate(t *testing.T, services *app.Services, guildContext *app.GuildStaffContext, input app.TemplateInput) *app.TemplateResponse {
	t.Helper()

	created, err := services.Templates.Create(context.Background(), guildContext, input)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return created
}

func caseCommandGuildContext(t *testing.T, services *app.Services) *app.GuildStaffContext {
	t.Helper()

	guildContext, err := services.Guilds.ResolveDiscordStaffContext(context.Background(), app.DiscordStaffContextInput{
		DiscordGuildID: "guild-1",
		DiscordUserID:  "owner-1",
		DisplayName:    "Owner",
	})
	if err != nil {
		t.Fatalf("resolve guild context: %v", err)
	}
	return guildContext
}

func caseAddInteraction(templateID, targetID, reason string, permissions uint64) *discordgo.InteractionCreate {
	options := []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:  "template",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: templateID,
		},
		{
			Name:  "user",
			Type:  discordgo.ApplicationCommandOptionUser,
			Value: targetID,
		},
	}
	if reason != "" {
		options = append(options, &discordgo.ApplicationCommandInteractionDataOption{
			Name:  "reason",
			Type:  discordgo.ApplicationCommandOptionString,
			Value: reason,
		})
	}

	return interaction(discordgo.InteractionApplicationCommand, permissions, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name:    "add",
			Type:    discordgo.ApplicationCommandOptionSubCommand,
			Options: options,
		},
	})
}

func templateAutocompleteInteraction(query string, permissions uint64) *discordgo.InteractionCreate {
	return interaction(discordgo.InteractionApplicationCommandAutocomplete, permissions, []*discordgo.ApplicationCommandInteractionDataOption{
		{
			Name: "add",
			Type: discordgo.ApplicationCommandOptionSubCommand,
			Options: []*discordgo.ApplicationCommandInteractionDataOption{
				{
					Name:    "template",
					Type:    discordgo.ApplicationCommandOptionString,
					Value:   query,
					Focused: true,
				},
			},
		},
	})
}

func interaction(interactionType discordgo.InteractionType, permissions uint64, options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:        "interaction-1",
			AppID:     "app-1",
			Type:      interactionType,
			GuildID:   "guild-1",
			ChannelID: "channel-1",
			Member: &discordgo.Member{
				User:        &discordgo.User{ID: "mod-1", Username: "mod", GlobalName: "Moderator"},
				Permissions: int64(permissions),
			},
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    "case",
				Options: options,
			},
		},
	}
}

func storeGuildID(t *testing.T, store *storage.Store, discordGuildID string) string {
	t.Helper()

	guild, err := store.GetGuildByDiscordID(context.Background(), discordGuildID)
	if err != nil {
		t.Fatalf("get guild: %v", err)
	}
	if guild == nil {
		t.Fatalf("expected guild %s", discordGuildID)
	}
	return guild.ID
}
