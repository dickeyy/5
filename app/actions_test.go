package app_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/structs"
)

type fakeActionClient struct {
	dmFailures []error
	dms        []fakeActionMessage
}

type fakeActionMessage struct {
	TargetID string
	Message  string
}

func (f *fakeActionClient) SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error) {
	_ = ctx
	if len(f.dmFailures) > 0 {
		err := f.dmFailures[0]
		f.dmFailures = f.dmFailures[1:]
		if err != nil {
			return nil, err
		}
	}
	f.dms = append(f.dms, fakeActionMessage{TargetID: discordUserID, Message: message})
	return map[string]any{"message_id": "dm-message-1"}, nil
}

func TestActionServiceProcessesSafeActions(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))
	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("safe-actions"))
	created, err := app.NewCaseService(store).Create(ctx, modContext, app.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	fakeDiscord := &fakeActionClient{}
	if err := app.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process actions: %v", err)
	}

	if len(fakeDiscord.dms) != 1 || fakeDiscord.dms[0].TargetID != "target-1" || fakeDiscord.dms[0].Message != "You received a warning in this server: No spam" {
		t.Fatalf("unexpected DMs: %+v", fakeDiscord.dms)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if len(actions) != 1 || actions[0].ActionType != structs.ActionSendDM || actions[0].Status != structs.ActionExecutionSucceeded {
		t.Fatalf("expected generated warning notification to succeed, got %+v", actions)
	}
	cases, err := store.ListCases(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list cases: %v", err)
	}
	if cases[0].Status != structs.CaseStatusCompleted {
		t.Fatalf("expected completed case, got %+v", cases[0])
	}
}

func TestActionServiceRetriesTransientFailure(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("retry-dm"))
	created, err := app.NewCaseService(store).Create(ctx, modContext, app.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if err := store.DB().Model(&structs.CaseActionExecution{}).Where("case_id = ?", created.ID).Updates(map[string]any{
		"max_retries":      1,
		"retry_backoff_ms": 1000,
	}).Error; err != nil {
		t.Fatalf("configure retry: %v", err)
	}

	fakeDiscord := &fakeActionClient{dmFailures: []error{
		app.DiscordActionError{Code: "rate_limited", Message: "rate limited", Retryable: true},
		nil,
	}}
	if err := app.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process first attempt: %v", err)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != structs.ActionExecutionRetrying || actions[0].NextRetryAt == nil {
		t.Fatalf("expected retrying action, got %+v", actions[0])
	}

	past := time.Now().UTC().Add(-time.Minute)
	if err := store.DB().Model(&structs.CaseActionExecution{}).Where("id = ?", actions[0].ID).Update("next_retry_at", past).Error; err != nil {
		t.Fatalf("make retry eligible: %v", err)
	}
	if err := app.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process second attempt: %v", err)
	}
	actions, err = store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != structs.ActionExecutionSucceeded || actions[0].AttemptCount != 2 {
		t.Fatalf("expected successful retry, got %+v", actions[0])
	}
}

func TestActionServiceFailureSkipsLaterActionsUnlessContinueOnError(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	input := actionTemplateInput("stop-on-error", []app.TemplateActionInput{
		{ActionType: structs.ActionTimeoutUser, Config: json.RawMessage(`{"duration_minutes":60}`), IdempotencyScope: "case"},
		{ActionType: structs.ActionKickUser, Config: json.RawMessage(`{}`), IdempotencyScope: "case"},
	})
	input.Levels[0].NotifyUser = false
	template := createAppTemplate(t, ctx, store, adminContext, input)
	created, err := app.NewCaseService(store).Create(ctx, modContext, app.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	fakeDiscord := &fakeActionClient{}
	if err := app.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process actions: %v", err)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != structs.ActionExecutionFailed || actions[1].Status != structs.ActionExecutionSkipped {
		t.Fatalf("expected failure to skip later action, got %+v", actions)
	}

	continueInput := actionTemplateInput("continue-on-error", []app.TemplateActionInput{
		{ActionType: structs.ActionTimeoutUser, Config: json.RawMessage(`{"duration_minutes":60}`), ContinueOnError: true, IdempotencyScope: "case"},
		{ActionType: structs.ActionKickUser, Config: json.RawMessage(`{}`), IdempotencyScope: "case"},
	})
	continueInput.Levels[0].NotifyUser = false
	continueTemplate := createAppTemplate(t, ctx, store, adminContext, continueInput)
	continued, err := app.NewCaseService(store).Create(ctx, modContext, app.CaseInput{
		TemplateID:          continueTemplate.ID,
		TargetDiscordUserID: "target-2",
	})
	if err != nil {
		t.Fatalf("create continuing case: %v", err)
	}
	fakeDiscord = &fakeActionClient{}
	if err := app.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, continued.ID); err != nil {
		t.Fatalf("process continuing actions: %v", err)
	}
	actions, err = store.ListCaseActionExecutions(ctx, continued.ID)
	if err != nil {
		t.Fatalf("list continuing actions: %v", err)
	}
	if actions[0].Status != structs.ActionExecutionFailed || actions[1].Status != structs.ActionExecutionFailed {
		t.Fatalf("expected continue_on_error to run later action, got %+v", actions)
	}
}

func TestActionServiceDoesNotNotifyForUnsupportedAction(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	unsupportedInput := actionTemplateInput("unsupported-notify", []app.TemplateActionInput{
		{ActionType: structs.ActionBanUser, Config: json.RawMessage(`{}`), NotifyUser: true, IdempotencyScope: "case"},
	})
	unsupportedInput.Levels[0].NotifyUser = false
	template := createAppTemplate(t, ctx, store, adminContext, unsupportedInput)
	created, err := app.NewCaseService(store).Create(ctx, modContext, app.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	fakeDiscord := &fakeActionClient{}
	if err := app.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process actions: %v", err)
	}
	if len(fakeDiscord.dms) != 0 {
		t.Fatalf("did not expect DM before unsupported ban action, got %+v", fakeDiscord.dms)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != structs.ActionExecutionFailed {
		t.Fatalf("expected unsupported action to fail, got %+v", actions[0])
	}
}

func actionTemplateInput(slug string, actions []app.TemplateActionInput) app.TemplateInput {
	input := validTemplateInput(slug)
	input.Levels[0].Actions = actions
	return input
}
