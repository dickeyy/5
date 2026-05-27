package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

func TestCaseServiceCreateFromTemplate(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("spam"))
	service := app.NewCaseService(store)

	created, err := service.Create(ctx, modContext, app.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
		Metadata:            json.RawMessage(`{"message_id":"123"}`),
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.ID == "" || created.CaseNumber != 1 {
		t.Fatalf("unexpected case response: %+v", created)
	}
	if created.Reason != "No spam" || created.Status != structs.CaseStatusOpen || created.Source != structs.CaseSourceAPI {
		t.Fatalf("unexpected case fields: %+v", created)
	}
	if len(created.Actions) != 1 || created.Actions[0].ActionType != structs.ActionSendDM {
		t.Fatalf("expected generated warning notification action, got %+v", created.Actions)
	}
	if created.SelectedLevel == nil || !created.SelectedLevel.IsDefault || created.SelectedLevel.MatchedCaseCount != 1 {
		t.Fatalf("expected selected default level, got %+v", created.SelectedLevel)
	}
	if created.Actions[0].Position != 0 || created.Actions[0].Status != structs.ActionExecutionPending {
		t.Fatalf("unexpected first action: %+v", created.Actions[0])
	}

	cases, err := store.ListCases(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list cases: %v", err)
	}
	var snapshot struct {
		Template struct {
			ID string `json:"id"`
		} `json:"template"`
		Actions []struct {
			ActionType structs.ActionType `json:"action_type"`
		} `json:"actions"`
		SelectedLevel struct {
			ID               string `json:"id"`
			IsDefault        bool   `json:"is_default"`
			MatchedCaseCount int64  `json:"matched_case_count"`
		} `json:"selected_level"`
	}
	if err := json.Unmarshal([]byte(cases[0].TemplateSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Template.ID != template.ID || len(snapshot.Actions) != 0 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.SelectedLevel.ID == "" || !snapshot.SelectedLevel.IsDefault || snapshot.SelectedLevel.MatchedCaseCount != 1 {
		t.Fatalf("unexpected selected level snapshot: %+v", snapshot.SelectedLevel)
	}
}

func TestCaseServiceRejectsUnavailableTemplates(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	templateService := app.NewTemplateService(store)
	caseService := app.NewCaseService(store)

	disabled := false
	disabledInput := validTemplateInput("disabled")
	disabledInput.Enabled = &disabled
	disabledTemplate := createAppTemplate(t, ctx, store, adminContext, disabledInput)

	archivedTemplate := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("archived"))
	if _, err := templateService.Archive(ctx, adminContext, archivedTemplate.ID); err != nil {
		t.Fatalf("archive template: %v", err)
	}

	tests := []struct {
		name       string
		templateID string
	}{
		{name: "missing", templateID: "missing-template"},
		{name: "disabled", templateID: disabledTemplate.ID},
		{name: "archived", templateID: archivedTemplate.ID},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := caseService.Create(ctx, modContext, app.CaseInput{
				TemplateID:          tt.templateID,
				TargetDiscordUserID: "target-1",
			})
			if !errors.Is(err, app.ErrCaseTemplateNotAvailable) {
				t.Fatalf("expected unavailable template error, got %v", err)
			}
		})
	}
}

func TestCaseServiceValidationFailures(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("spam"))
	service := app.NewCaseService(store)

	tests := []struct {
		name  string
		input app.CaseInput
	}{
		{name: "missing template", input: app.CaseInput{TargetDiscordUserID: "target-1"}},
		{name: "missing target", input: app.CaseInput{TemplateID: template.ID}},
		{name: "invalid source", input: app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1", Source: "invalid"}},
		{name: "invalid metadata", input: app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1", Metadata: json.RawMessage(`[]`)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.Create(ctx, modContext, tt.input)
			if !errors.Is(err, app.ErrCaseValidation) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestCaseServiceRejectsEmptyFinalReason(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	guildContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	created, err := store.CreateCaseTemplate(ctx, storage.CreateCaseTemplateParams{
		Template: structs.CaseTemplate{
			GuildID:                guildContext.Guild.ID,
			Slug:                   "empty-reason",
			Name:                   "Empty Reason",
			ReasonTemplate:         " ",
			DefaultSeverity:        structs.CaseSeverityMedium,
			Enabled:                true,
			CreatedByDiscordUserID: "admin-1",
			UpdatedByDiscordUserID: "admin-1",
		},
		Levels: []storage.ExpandedCaseTemplateLevel{
			{
				Level: structs.CaseTemplateLevel{Position: 1, Name: "Default", IsDefault: true, Enabled: true},
			},
		},
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	_, err = app.NewCaseService(store).Create(ctx, guildContext, app.CaseInput{
		TemplateID:          created.Template.ID,
		TargetDiscordUserID: "target-1",
	})
	if !errors.Is(err, app.ErrCaseValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCaseServiceCreatesActionlessWarningCase(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	input := validTemplateInput("silent-warning")
	input.Levels[0].NotifyUser = false
	template := createAppTemplate(t, ctx, store, adminContext, input)

	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if len(created.Actions) != 0 {
		t.Fatalf("expected no action rows for silent warning case, got %+v", created.Actions)
	}
	events, err := store.ListCaseEvents(ctx, created.ID)
	if err != nil {
		t.Fatalf("list case events: %v", err)
	}
	if len(events) != 1 || events[0].EventType != structs.CaseEventCreated {
		t.Fatalf("expected only case created event, got %+v", events)
	}
	audits, err := store.ListAuditLogEntries(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if audits[len(audits)-1].Action != "case.create" {
		t.Fatalf("expected case.create as warning audit trail, got %+v", audits)
	}
}

func TestCaseServicePermissionFailures(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	noCreateContext := *modContext
	noCreateContext.Permissions = map[structs.PermissionAction]bool{structs.PermissionActionCaseCreate: false}
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("spam"))
	_, err := service.Create(ctx, &noCreateContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if !errors.Is(err, app.ErrCasePermissionDenied) {
		t.Fatalf("expected case.create permission error, got %v", err)
	}
}

func TestCaseServiceSelectsEscalationLevelFromSameTemplateHistory(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("spam"))
	for i := 0; i < 2; i++ {
		created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
		if err != nil {
			t.Fatalf("create prior case %d: %v", i+1, err)
		}
		if created.SelectedLevel == nil || !created.SelectedLevel.IsDefault {
			t.Fatalf("expected prior case to use default level, got %+v", created.SelectedLevel)
		}
	}

	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.SelectedLevel == nil || created.SelectedLevel.IsDefault || created.SelectedLevel.TriggerCaseCount != 3 || created.SelectedLevel.MatchedCaseCount != 3 {
		t.Fatalf("expected repeat spam level, got %+v", created.SelectedLevel)
	}
	if len(created.Actions) != 1 || created.Actions[0].ActionType != structs.ActionTimeoutUser {
		t.Fatalf("expected selected escalation action only, got %+v", created.Actions)
	}

	cases, err := store.ListCases(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list cases: %v", err)
	}
	var snapshot struct {
		SelectedLevel struct {
			TriggerCaseCount int   `json:"trigger_case_count"`
			MatchedCaseCount int64 `json:"matched_case_count"`
		} `json:"selected_level"`
		Actions []struct {
			ActionType structs.ActionType `json:"action_type"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(cases[2].TemplateSnapshotJSON), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.SelectedLevel.TriggerCaseCount != 3 || snapshot.SelectedLevel.MatchedCaseCount != 3 || len(snapshot.Actions) != 1 || snapshot.Actions[0].ActionType != structs.ActionTimeoutUser {
		t.Fatalf("unexpected escalation snapshot: %+v", snapshot)
	}
}

func TestCaseServiceHighestMatchingLevelWins(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	input := validTemplateInput("spam")
	input.Levels = append(input.Levels, app.TemplateLevelInput{
		Name:             "Early repeat",
		Position:         3,
		TriggerCaseCount: 2,
	})
	template := createAppTemplate(t, ctx, store, adminContext, input)

	for i := 0; i < 2; i++ {
		if _, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"}); err != nil {
			t.Fatalf("create prior case %d: %v", i+1, err)
		}
	}
	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.SelectedLevel == nil || created.SelectedLevel.TriggerCaseCount != 3 {
		t.Fatalf("expected highest trigger threshold to win, got %+v", created.SelectedLevel)
	}
}

func TestCaseServiceLevelTieBreaksByHigherPosition(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	input := validTemplateInput("spam")
	input.Levels[1].Name = "Lower position"
	input.Levels[1].Position = 2
	input.Levels[1].TriggerCaseCount = 1
	input.Levels = append(input.Levels, app.TemplateLevelInput{
		Name:             "Higher position",
		Position:         3,
		TriggerCaseCount: 1,
		Actions: []app.TemplateActionInput{
			{ActionType: structs.ActionKickUser, Config: json.RawMessage(`{"delete_message_seconds":0}`), IdempotencyScope: "case"},
		},
	})
	template := createAppTemplate(t, ctx, store, adminContext, input)

	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.SelectedLevel == nil || created.SelectedLevel.Position != 3 {
		t.Fatalf("expected higher position tie-breaker to win, got %+v", created.SelectedLevel)
	}
	if len(created.Actions) != 1 || created.Actions[0].ActionType != structs.ActionKickUser {
		t.Fatalf("expected higher position level action, got %+v", created.Actions)
	}
}

func TestCaseServiceLevelWindowsAndFilters(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	input := validTemplateInput("spam")
	input.Levels[1].TriggerCaseCount = 2
	input.Levels[1].WindowMinutes = 60
	template := createAppTemplate(t, ctx, store, adminContext, input)

	old, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create old case: %v", err)
	}
	oldTime := time.Now().UTC().Add(-2 * time.Hour)
	if err := store.DB().Model(&structs.Case{}).Where("id = ?", old.ID).Update("created_at", oldTime).Error; err != nil {
		t.Fatalf("age old case: %v", err)
	}

	otherTemplate := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("other-template"))
	if _, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: otherTemplate.ID, TargetDiscordUserID: "target-1"}); err != nil {
		t.Fatalf("create other template case: %v", err)
	}
	if _, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-2"}); err != nil {
		t.Fatalf("create other target case: %v", err)
	}

	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.SelectedLevel == nil || !created.SelectedLevel.IsDefault || created.SelectedLevel.MatchedCaseCount != 2 {
		t.Fatalf("expected default because matching prior cases are outside filters/window, got %+v", created.SelectedLevel)
	}
}

func TestCaseServiceVoidedCasesDoNotCount(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	input := validTemplateInput("spam")
	input.Levels[1].TriggerCaseCount = 2
	template := createAppTemplate(t, ctx, store, adminContext, input)

	prior, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create prior case: %v", err)
	}
	if err := store.DB().Model(&structs.Case{}).Where("id = ?", prior.ID).Update("status", structs.CaseStatusVoided).Error; err != nil {
		t.Fatalf("void prior case: %v", err)
	}

	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.SelectedLevel == nil || !created.SelectedLevel.IsDefault || created.SelectedLevel.MatchedCaseCount != 1 {
		t.Fatalf("expected voided prior case to be ignored, got %+v", created.SelectedLevel)
	}
}

func TestCaseServiceDisabledLevelsDoNotMatch(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)

	input := validTemplateInput("spam")
	input.Levels[1].TriggerCaseCount = 1
	disabled := false
	input.Levels[1].Enabled = &disabled
	template := createAppTemplate(t, ctx, store, adminContext, input)

	created, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if created.SelectedLevel == nil || !created.SelectedLevel.IsDefault || len(created.Actions) != 1 || created.Actions[0].ActionType != structs.ActionSendDM {
		t.Fatalf("expected disabled escalation to be ignored, got level=%+v actions=%+v", created.SelectedLevel, created.Actions)
	}
}

func TestCaseServiceDashboardReads(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("spam"))

	first, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create first case: %v", err)
	}
	second, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-2"})
	if err != nil {
		t.Fatalf("create second case: %v", err)
	}

	list, err := service.List(ctx, modContext, app.CaseListInput{Limit: "10"})
	if err != nil {
		t.Fatalf("list cases: %v", err)
	}
	if list.Total != 2 || len(list.Cases) != 2 || list.Cases[0].ID != second.ID || list.Cases[1].ID != first.ID {
		t.Fatalf("expected newest-first case list, got %+v", list)
	}
	if list.Cases[0].SelectedLevel == nil {
		t.Fatalf("expected selected level in case list response")
	}

	detail, err := service.Get(ctx, modContext, "1")
	if err != nil {
		t.Fatalf("get case detail: %v", err)
	}
	if detail.ID != first.ID || detail.TemplateSnapshot == nil || len(detail.Events) != 1 || len(detail.Actions) != 1 {
		t.Fatalf("unexpected case detail: %+v", detail)
	}
	if detail.Actions[0].ActionType != structs.ActionSendDM || len(detail.Actions[0].Attempts) != 0 {
		t.Fatalf("unexpected detail actions: %+v", detail.Actions)
	}

	profile, err := service.UserHistory(ctx, modContext, "target-1", app.CaseListInput{Limit: "10"})
	if err != nil {
		t.Fatalf("user history: %v", err)
	}
	if profile.Total != 1 || len(profile.Cases) != 1 || profile.Summary.Total != 1 || profile.Summary.ByStatus[string(structs.CaseStatusOpen)] != 1 {
		t.Fatalf("unexpected profile response: %+v", profile)
	}
	if profile.Summary.ByTemplate[template.ID] != 1 {
		t.Fatalf("expected profile summary by template, got %+v", profile.Summary.ByTemplate)
	}
}

func TestCaseServiceReadValidationAndPermissions(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	service := app.NewCaseService(store)
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("spam"))
	if _, err := service.Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"}); err != nil {
		t.Fatalf("create case: %v", err)
	}

	_, err := service.List(ctx, modContext, app.CaseListInput{Limit: "0"})
	if !errors.Is(err, app.ErrCaseValidation) {
		t.Fatalf("expected limit validation error, got %v", err)
	}
	_, err = service.List(ctx, modContext, app.CaseListInput{Status: "not-a-status"})
	if !errors.Is(err, app.ErrCaseValidation) {
		t.Fatalf("expected status validation error, got %v", err)
	}
	_, err = service.Get(ctx, modContext, "missing")
	if !errors.Is(err, app.ErrCaseNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}

	noReadContext := *modContext
	noReadContext.Permissions = map[structs.PermissionAction]bool{structs.PermissionActionCaseCreate: false}
	_, err = service.List(ctx, &noReadContext, app.CaseListInput{})
	if !errors.Is(err, app.ErrCasePermissionDenied) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestCaseServiceTraceIDsPropagateToCaseActionsAndAudit(t *testing.T) {
	ctx := app.ContextWithTrace(context.Background(), "req-case-1", "corr-case-1")
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("trace-spam"))

	created, err := app.NewCaseService(store).Create(ctx, modContext, app.CaseInput{TemplateID: template.ID, TargetDiscordUserID: "target-1"})
	if err != nil {
		t.Fatalf("create traced case: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("expected created case id")
	}

	caseModel, err := store.GetCaseByIDOrNumber(ctx, modContext.Guild.ID, created.ID)
	if err != nil {
		t.Fatalf("get traced case: %v", err)
	}
	if caseModel == nil || caseModel.CorrelationID != "corr-case-1" {
		t.Fatalf("expected case correlation id, got %+v", caseModel)
	}

	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list traced actions: %v", err)
	}
	if len(actions) != 1 || actions[0].CorrelationID != "corr-case-1" {
		t.Fatalf("expected action correlation id, got %+v", actions)
	}

	audits, err := store.ListAuditLogEntries(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	var foundCaseAudit bool
	for _, audit := range audits {
		if audit.Action == "case.create" {
			foundCaseAudit = true
			if audit.RequestID != "req-case-1" || audit.CorrelationID != "corr-case-1" {
				t.Fatalf("expected traced case audit, got %+v", audit)
			}
		}
	}
	if !foundCaseAudit {
		t.Fatalf("expected case.create audit in %+v", audits)
	}
}

func createAppTemplate(t *testing.T, ctx context.Context, store *storage.Store, guildContext *app.GuildStaffContext, input app.TemplateInput) *app.TemplateResponse {
	t.Helper()

	created, err := app.NewTemplateService(store).Create(ctx, guildContext, input)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	return created
}
