package quack_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
	"github.com/quackdiscord/bot/internal/store"
)

func TestTemplateServiceCreateNormalizesActionsAndAudits(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	guildContext := templateGuildContext(t, store, "guild-1", "user-1", uint64(discordgo.PermissionManageGuild))
	service := quack.NewTemplateService(store)

	created, err := service.Create(ctx, guildContext, validTemplateInput("spam"))
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected template id")
	}
	if len(created.Levels) != 2 || !created.Levels[0].IsDefault {
		t.Fatalf("expected default and escalation levels, got %+v", created.Levels)
	}
	if len(created.Levels[0].Actions) != 0 {
		t.Fatalf("expected actionless default warning level, got %+v", created.Levels[0].Actions)
	}
	if !created.Levels[0].NotifyUser || created.Levels[0].NotificationType != string(model.NotificationWarning) {
		t.Fatalf("expected default level to notify as warning, got %+v", created.Levels[0])
	}

	audits, err := store.ListAuditLogEntries(ctx, guildContext.Guild.ID)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(audits) != 1 || audits[0].Action != "case_template.create" || audits[0].Result != model.AuditResultSuccess {
		t.Fatalf("expected successful create audit, got %+v", audits)
	}
}

func TestTemplateServiceValidationFailures(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	guildContext := templateGuildContext(t, store, "guild-1", "user-1", uint64(discordgo.PermissionManageGuild))
	service := quack.NewTemplateService(store)

	tests := []struct {
		name string
		edit func(*quack.TemplateInput)
	}{
		{name: "invalid slug", edit: func(input *quack.TemplateInput) { input.Slug = "Invalid Slug" }},
		{name: "empty reason", edit: func(input *quack.TemplateInput) { input.ReasonTemplate = "" }},
		{name: "invalid action", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].ActionType = "explode_user" }},
		{name: "record warning action", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].ActionType = "record_warning" }},
		{name: "send dm action", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].ActionType = model.ActionSendDM }},
		{name: "invalid config", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].Config = json.RawMessage(`[]`) }},
		{name: "invalid notification type", edit: func(input *quack.TemplateInput) {
			input.Levels[1].Actions[0].NotifyUser = true
			input.Levels[1].Actions[0].NotificationType = "notice"
		}},
		{name: "invalid level notification type", edit: func(input *quack.TemplateInput) { input.Levels[0].NotificationType = "notice" }},
		{name: "negative retry", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].MaxRetries = -1 }},
		{name: "negative backoff", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].RetryBackoffMS = -1 }},
		{name: "negative timeout", edit: func(input *quack.TemplateInput) { input.Levels[1].Actions[0].TimeoutMS = -1 }},
		{name: "no default level", edit: func(input *quack.TemplateInput) { input.Levels[0].IsDefault = false }},
		{name: "two default levels", edit: func(input *quack.TemplateInput) { input.Levels[1].IsDefault = true }},
		{name: "default level trigger", edit: func(input *quack.TemplateInput) { input.Levels[0].TriggerCaseCount = 1 }},
		{name: "escalation without trigger", edit: func(input *quack.TemplateInput) { input.Levels[1].TriggerCaseCount = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validTemplateInput("spam-" + strings.ReplaceAll(tt.name, " ", "-"))
			tt.edit(&input)
			_, err := service.Create(ctx, guildContext, input)
			if !errors.Is(err, quack.ErrTemplateValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestTemplateServiceUpdateAndArchiveAudit(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	guildContext := templateGuildContext(t, store, "guild-1", "user-1", uint64(discordgo.PermissionManageGuild))
	service := quack.NewTemplateService(store)

	created, err := service.Create(ctx, guildContext, validTemplateInput("spam"))
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	update := validTemplateInput("spam-updated")
	update.Name = "Spam Updated"
	update.Levels[1].Actions = nil
	updated, err := service.Update(ctx, guildContext, created.ID, update)
	if err != nil {
		t.Fatalf("update template: %v", err)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("expected version increment, got %d then %d", created.Version, updated.Version)
	}
	if len(updated.Levels) != 2 || len(updated.Levels[1].Actions) != 0 {
		t.Fatalf("expected update to replace levels and actions, got %+v", updated.Levels)
	}

	archived, err := service.Archive(ctx, guildContext, created.ID)
	if err != nil {
		t.Fatalf("archive template: %v", err)
	}
	if archived.Enabled || archived.ArchivedAt == nil {
		t.Fatalf("expected archived disabled template")
	}

	audits, err := store.ListAuditLogEntries(ctx, guildContext.Guild.ID)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(audits) != 3 {
		t.Fatalf("expected create/update/archive audits, got %+v", audits)
	}
}

func TestTemplateServiceGuildBoundary(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	guildOne := templateGuildContext(t, store, "guild-1", "user-1", uint64(discordgo.PermissionManageGuild))
	guildTwo := templateGuildContext(t, store, "guild-2", "user-1", uint64(discordgo.PermissionManageGuild))
	service := quack.NewTemplateService(store)

	created, err := service.Create(ctx, guildOne, validTemplateInput("spam"))
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	_, err = service.Update(ctx, guildTwo, created.ID, validTemplateInput("spam"))
	if !errors.Is(err, quack.ErrTemplateNotFound) {
		t.Fatalf("expected not found across guild boundary, got %v", err)
	}
}

func TestTemplateServiceSlugUniqueness(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	guildContext := templateGuildContext(t, store, "guild-1", "user-1", uint64(discordgo.PermissionManageGuild))
	service := quack.NewTemplateService(store)

	if _, err := service.Create(ctx, guildContext, validTemplateInput("spam")); err != nil {
		t.Fatalf("create template: %v", err)
	}
	_, err := service.Create(ctx, guildContext, validTemplateInput("spam"))
	if !errors.Is(err, quack.ErrTemplateValidation) {
		t.Fatalf("expected duplicate slug validation, got %v", err)
	}
}

func templateGuildContext(t *testing.T, store *store.Store, discordGuildID, userID string, permissionBits uint64) *quack.GuildStaffContext {
	t.Helper()

	service := quack.NewGuildService(store, fakeDiscordClient{
		userGuilds: []quack.DiscordUserGuild{{ID: discordGuildID, Owner: permissionBits == 0, Permissions: permissionBits}},
		botGuild:   &quack.DiscordBotGuild{ID: discordGuildID, Name: "Guild", OwnerID: "owner-1"},
	})
	guildContext, err := service.ResolveStaffContext(context.Background(), testSession(userID), discordGuildID)
	if err != nil {
		t.Fatalf("resolve guild context: %v", err)
	}
	return guildContext
}

func validTemplateInput(slug string) quack.TemplateInput {
	return quack.TemplateInput{
		Slug:           slug,
		Name:           "Spam",
		Description:    "Spam template",
		ReasonTemplate: "No spam",
		Appealable:     true,
		Levels: []quack.TemplateLevelInput{
			{
				Name:       "Default",
				Position:   1,
				IsDefault:  true,
				NotifyUser: true,
			},
			{
				Name:             "Repeat spam",
				Position:         2,
				TriggerCaseCount: 3,
				WindowMinutes:    1440,
				Actions: []quack.TemplateActionInput{
					{
						ActionType:       model.ActionTimeoutUser,
						Config:           json.RawMessage(`{"duration_minutes":60}`),
						IdempotencyScope: "case",
					},
				},
			},
		},
	}
}
