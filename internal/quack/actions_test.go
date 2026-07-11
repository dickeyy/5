package quack_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
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
	created, err := quack.NewCaseService(store).Create(ctx, modContext, quack.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	fakeDiscord := &fakeActionClient{}
	if err := quack.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process actions: %v", err)
	}

	if len(fakeDiscord.dms) != 1 || fakeDiscord.dms[0].TargetID != "target-1" || fakeDiscord.dms[0].Message != "You received a warning in this server: No spam" {
		t.Fatalf("unexpected DMs: %+v", fakeDiscord.dms)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if len(actions) != 1 || actions[0].ActionType != model.ActionSendDM || actions[0].Status != model.ActionExecutionSucceeded {
		t.Fatalf("expected generated warning notification to succeed, got %+v", actions)
	}
	cases, err := store.ListCases(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list cases: %v", err)
	}
	if cases[0].Status != model.CaseStatusCompleted {
		t.Fatalf("expected completed case, got %+v", cases[0])
	}
}

func TestActionServiceRetriesTransientFailure(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	template := createAppTemplate(t, ctx, store, adminContext, validTemplateInput("retry-dm"))
	created, err := quack.NewCaseService(store).Create(ctx, modContext, quack.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	if err := store.DB().Model(&model.CaseActionExecution{}).Where("case_id = ?", created.ID).Updates(map[string]any{
		"max_retries":      1,
		"retry_backoff_ms": 1000,
	}).Error; err != nil {
		t.Fatalf("configure retry: %v", err)
	}

	fakeDiscord := &fakeActionClient{dmFailures: []error{
		quack.DiscordActionError{Code: "rate_limited", Message: "rate limited", Retryable: true},
		nil,
	}}
	if err := quack.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process first attempt: %v", err)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != model.ActionExecutionRetrying || actions[0].NextRetryAt == nil {
		t.Fatalf("expected retrying action, got %+v", actions[0])
	}

	past := time.Now().UTC().Add(-time.Minute)
	if err := store.DB().Model(&model.CaseActionExecution{}).Where("id = ?", actions[0].ID).Update("next_retry_at", past).Error; err != nil {
		t.Fatalf("make retry eligible: %v", err)
	}
	if err := quack.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process second attempt: %v", err)
	}
	actions, err = store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != model.ActionExecutionSucceeded || actions[0].AttemptCount != 2 {
		t.Fatalf("expected successful retry, got %+v", actions[0])
	}
}

func TestActionServiceFailureSkipsLaterActionsUnlessContinueOnError(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	input := actionTemplateInput("stop-on-error", []quack.TemplateActionInput{
		{ActionType: model.ActionTimeoutUser, Config: json.RawMessage(`{"duration_minutes":60}`), IdempotencyScope: "case"},
		{ActionType: model.ActionKickUser, Config: json.RawMessage(`{}`), IdempotencyScope: "case"},
	})
	input.Levels[0].NotifyUser = false
	template := createAppTemplate(t, ctx, store, adminContext, input)
	created, err := quack.NewCaseService(store).Create(ctx, modContext, quack.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}
	fakeDiscord := &fakeActionClient{}
	if err := quack.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process actions: %v", err)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != model.ActionExecutionFailed || actions[1].Status != model.ActionExecutionSkipped {
		t.Fatalf("expected failure to skip later action, got %+v", actions)
	}

	continueInput := actionTemplateInput("continue-on-error", []quack.TemplateActionInput{
		{ActionType: model.ActionTimeoutUser, Config: json.RawMessage(`{"duration_minutes":60}`), ContinueOnError: true, IdempotencyScope: "case"},
		{ActionType: model.ActionKickUser, Config: json.RawMessage(`{}`), IdempotencyScope: "case"},
	})
	continueInput.Levels[0].NotifyUser = false
	continueTemplate := createAppTemplate(t, ctx, store, adminContext, continueInput)
	continued, err := quack.NewCaseService(store).Create(ctx, modContext, quack.CaseInput{
		TemplateID:          continueTemplate.ID,
		TargetDiscordUserID: "target-2",
	})
	if err != nil {
		t.Fatalf("create continuing case: %v", err)
	}
	fakeDiscord = &fakeActionClient{}
	if err := quack.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, continued.ID); err != nil {
		t.Fatalf("process continuing actions: %v", err)
	}
	actions, err = store.ListCaseActionExecutions(ctx, continued.ID)
	if err != nil {
		t.Fatalf("list continuing actions: %v", err)
	}
	if actions[0].Status != model.ActionExecutionFailed || actions[1].Status != model.ActionExecutionFailed {
		t.Fatalf("expected continue_on_error to run later action, got %+v", actions)
	}
}

func TestActionServiceDoesNotNotifyForUnsupportedAction(t *testing.T) {
	ctx := quack.ContextWithTrace(context.Background(), "req-action-1", "corr-action-1")
	store := newMigratedStore(t)
	adminContext := templateGuildContext(t, store, "guild-1", "admin-1", uint64(discordgo.PermissionManageGuild))
	modContext := templateGuildContext(t, store, "guild-1", "mod-1", uint64(discordgo.PermissionModerateMembers))

	unsupportedInput := actionTemplateInput("unsupported-notify", []quack.TemplateActionInput{
		{ActionType: model.ActionBanUser, Config: json.RawMessage(`{}`), NotifyUser: true, IdempotencyScope: "case"},
	})
	unsupportedInput.Levels[0].NotifyUser = false
	template := createAppTemplate(t, ctx, store, adminContext, unsupportedInput)
	created, err := quack.NewCaseService(store).Create(ctx, modContext, quack.CaseInput{
		TemplateID:          template.ID,
		TargetDiscordUserID: "target-1",
	})
	if err != nil {
		t.Fatalf("create case: %v", err)
	}

	fakeDiscord := &fakeActionClient{}
	if err := quack.NewActionService(store, fakeDiscord).ProcessCaseActions(ctx, created.ID); err != nil {
		t.Fatalf("process actions: %v", err)
	}
	if len(fakeDiscord.dms) != 0 {
		t.Fatalf("did not expect DM before unsupported ban action, got %+v", fakeDiscord.dms)
	}
	actions, err := store.ListCaseActionExecutions(ctx, created.ID)
	if err != nil {
		t.Fatalf("list actions: %v", err)
	}
	if actions[0].Status != model.ActionExecutionFailed {
		t.Fatalf("expected unsupported action to fail, got %+v", actions[0])
	}
	if actions[0].LastErrorCode != "action_not_implemented" || actions[0].NextRetryAt != nil {
		t.Fatalf("expected visible non-retryable unsupported action, got %+v", actions[0])
	}
	attempts, err := store.ListCaseActionAttempts(ctx, []string{actions[0].ID})
	if err != nil {
		t.Fatalf("list attempts: %v", err)
	}
	if len(attempts) != 1 || attempts[0].ErrorCode != "action_not_implemented" {
		t.Fatalf("expected failed unsupported attempt, got %+v", attempts)
	}
	audits, err := store.ListAuditLogEntries(ctx, modContext.Guild.ID)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	var failureAudit *model.AuditLogEntry
	for i := range audits {
		if audits[i].Action == "case_action.failed" {
			failureAudit = &audits[i]
			break
		}
	}
	if failureAudit == nil || failureAudit.RequestID != "req-action-1" || failureAudit.CorrelationID != "corr-action-1" {
		t.Fatalf("expected traced action failure audit, got %+v", audits)
	}
}

func actionTemplateInput(slug string, actions []quack.TemplateActionInput) quack.TemplateInput {
	input := validTemplateInput(slug)
	input.Levels[0].Actions = actions
	return input
}
