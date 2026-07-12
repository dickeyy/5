package quack_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

func TestAuditServiceListPermissionsAndFilters(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionAdministrator))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	entries := []model.AuditLogEntry{
		{GuildID: adminContext.Guild.ID, ActorDiscordUserID: "actor-1", ActorPermissionBits: uint64(discordgo.PermissionManageGuild), Source: model.AuditSourceAPI, Action: "case.create", ResourceType: "case", ResourceID: "case-1", Result: model.AuditResultSuccess, MetadataJSON: "{}"},
		{GuildID: adminContext.Guild.ID, ActorDiscordUserID: "actor-2", Source: model.AuditSourceSystem, Action: "case_action.failed", ResourceType: "case_action_execution", ResourceID: "action-1", Result: model.AuditResultFailure, MetadataJSON: "{}"},
	}
	for i := range entries {
		if err := store.CreateAuditLogEntry(ctx, &entries[i]); err != nil {
			t.Fatalf("create audit %d: %v", i, err)
		}
	}

	service := quack.NewAuditService(store)
	moderatorList, err := service.List(ctx, modContext, quack.AuditListInput{})
	if err != nil || moderatorList.Total != 2 {
		t.Fatalf("expected moderator complete audit access, list=%+v err=%v", moderatorList, err)
	}

	list, err := service.List(ctx, adminContext, quack.AuditListInput{Result: string(model.AuditResultFailure), Limit: "10"})
	if err != nil {
		t.Fatalf("list audit entries: %v", err)
	}
	if list.Total != 1 || len(list.Entries) != 1 || list.Entries[0].Action != "case_action.failed" {
		t.Fatalf("unexpected audit list: %+v", list)
	}

	_, err = service.List(ctx, adminContext, quack.AuditListInput{Result: "partial"})
	if !errors.Is(err, quack.ErrAuditValidation) {
		t.Fatalf("expected audit validation error, got %v", err)
	}
}
