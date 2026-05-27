package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/structs"
)

func TestAuditServiceListPermissionsAndFilters(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	entries := []structs.AuditLogEntry{
		{GuildID: adminContext.Guild.ID, ActorDiscordUserID: "actor-1", ActorPermissionBits: uint64(discordgo.PermissionManageGuild), Source: structs.AuditSourceAPI, Action: "case.create", ResourceType: "case", ResourceID: "case-1", Result: structs.AuditResultSuccess, MetadataJSON: "{}"},
		{GuildID: adminContext.Guild.ID, ActorDiscordUserID: "actor-2", Source: structs.AuditSourceSystem, Action: "case_action.failed", ResourceType: "case_action_execution", ResourceID: "action-1", Result: structs.AuditResultFailure, MetadataJSON: "{}"},
	}
	for i := range entries {
		if err := store.CreateAuditLogEntry(ctx, &entries[i]); err != nil {
			t.Fatalf("create audit %d: %v", i, err)
		}
	}

	service := app.NewAuditService(store)
	_, err := service.List(ctx, modContext, app.AuditListInput{})
	if !errors.Is(err, app.ErrAuditPermissionDenied) {
		t.Fatalf("expected moderator audit permission error, got %v", err)
	}

	list, err := service.List(ctx, adminContext, app.AuditListInput{Result: string(structs.AuditResultFailure), Limit: "10"})
	if err != nil {
		t.Fatalf("list audit entries: %v", err)
	}
	if list.Total != 1 || len(list.Entries) != 1 || list.Entries[0].Action != "case_action.failed" {
		t.Fatalf("unexpected audit list: %+v", list)
	}

	_, err = service.List(ctx, adminContext, app.AuditListInput{Result: "partial"})
	if !errors.Is(err, app.ErrAuditValidation) {
		t.Fatalf("expected audit validation error, got %v", err)
	}
}
