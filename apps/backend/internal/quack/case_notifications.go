package quack

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	actionmods "github.com/quackdiscord/bot/internal/quack/actionmods"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// prepareNotification opens a DM channel before an irreversible membership change without coupling preparation failure to enforcement.
func (s *ActionService) prepareNotification(ctx context.Context, item model.Case) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	notification, err := s.store.GetCaseNotification(ctx, item.ID)
	if err != nil {
		slog.ErrorContext(ctx, "Could not load notification before enforcement", "case_id", item.ID, "error", err)
	}
	if err != nil || notification == nil || notification.Status != model.NotificationPending {
		return
	}
	prepared, ok := s.discord.(DiscordPreparedDMClient)
	if !ok {
		_ = s.store.PrepareCaseNotification(ctx, item.ID, "", "prepared DM adapter is unavailable")
		return
	}
	channelID, prepareErr := prepared.PrepareDM(ctx, item.TargetDiscordUserID)
	message := ""
	if prepareErr != nil {
		message = redactDiscordError(prepareErr)
	}
	if err := s.store.PrepareCaseNotification(ctx, item.ID, channelID, message); err != nil {
		slog.ErrorContext(ctx, "Could not record prepared notification", "case_id", item.ID, "error", err)
	}
}

// processNotification renders and attempts the one case-level notification after enforcement reaches a terminal outcome.
func (s *ActionService) processNotification(ctx context.Context, workerID, caseID string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	claimed, err := s.store.ClaimCaseNotification(ctx, model.ClaimCaseNotificationParams{CaseID: caseID, WorkerID: workerID})
	if err != nil || claimed == nil {
		return err
	}
	item, err := s.store.GetCaseByID(ctx, caseID)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrCaseNotFound
	}
	guild, err := s.store.GetGuildByID(ctx, item.GuildID)
	if err != nil {
		return err
	}
	settings, err := s.store.GetGuildSettings(ctx, item.GuildID)
	if err != nil {
		return err
	}
	actions, err := s.store.ListCaseActionExecutions(ctx, item.ID)
	if err != nil {
		return err
	}
	message := renderCaseNotification(*item, guild, settings, actions)
	if err := s.store.BeginCaseNotificationDelivery(ctx, claimed.ID, claimed.LeaseToken); err != nil {
		return err
	}
	var response map[string]any
	var sendErr error
	appealable := caseSnapshotAppealable(item.TemplateSnapshotJSON)
	if appealable && s.dashboardBaseURL != "" {
		if client, ok := s.discord.(DiscordCaseNotificationClient); ok {
			response, sendErr = client.SendCaseNotification(ctx, item.TargetDiscordUserID, claimed.PreparedChannelDiscordID, message, s.dashboardBaseURL, item.GuildID, item.ID)
		} else {
			sendErr = errors.New("Discord appeal notification adapter is unavailable")
		}
	} else if claimed.PreparedChannelDiscordID != "" {
		if prepared, ok := s.discord.(DiscordPreparedDMClient); ok {
			response, sendErr = prepared.SendPreparedDM(ctx, claimed.PreparedChannelDiscordID, message)
		} else {
			sendErr = errors.New("prepared DM adapter is unavailable")
		}
	} else if s.discord != nil {
		response, sendErr = s.discord.SendDM(ctx, item.TargetDiscordUserID, message)
	} else {
		sendErr = errors.New("Discord action client is unavailable")
	}
	params := model.CompleteCaseNotificationParams{NotificationID: claimed.ID, LeaseToken: claimed.LeaseToken, WorkerID: workerID, RenderedMessage: message, PreparedChannelDiscordID: claimed.PreparedChannelDiscordID}
	if sendErr != nil {
		result := actionmods.ResultFromError(sendErr)
		params.Status = model.NotificationFailed
		params.ErrorCode = result.ErrorCode
		params.ErrorMessage = result.Error
		params.EventType = model.CaseEventNotificationFailed
	} else {
		params.Status = model.NotificationSent
		params.EventType = model.CaseEventNotificationSent
		if id, ok := response["message_id"].(string); ok {
			params.DeliveryMessageDiscordID = id
		}
	}
	if err := s.store.CompleteCaseNotification(ctx, params); err != nil {
		return fmt.Errorf("record case notification result: %w", err)
	}
	level := slog.LevelInfo
	if sendErr != nil {
		level = slog.LevelWarn
	}
	slog.Log(ctx, level, "Case notification recorded", "case_id", caseID, "status", params.Status, "error_code", params.ErrorCode)
	return nil
}

// renderCaseNotification builds the bounded product-owned message without executable guild templates.
func renderCaseNotification(item model.Case, guild *model.Guild, settings *model.GuildSettings, actions []model.CaseActionExecution) string {
	guildName := "this server"
	if guild != nil && strings.TrimSpace(guild.Name) != "" {
		guildName = guild.Name
	}
	parts := []string{}
	if settings != nil && strings.TrimSpace(settings.NotificationIntroduction) != "" {
		parts = append(parts, truncateRunes(strings.TrimSpace(settings.NotificationIntroduction), 150))
	}
	parts = append(parts, fmt.Sprintf("Moderation case #%d in %s", item.CaseNumber, guildName), "Reason: "+truncateRunes(item.Reason, 200))
	for _, value := range parseCaseContextValues(item.ContextValuesJSON) {
		if value.Value != nil {
			parts = append(parts, fmt.Sprintf("%s: %s", truncateRunes(value.Label, 40), truncateRunes(fmt.Sprint(value.Value), 50)))
		}
	}
	outcome := "No Discord enforcement action was configured."
	if len(actions) > 0 {
		action := actions[0]
		outcome = fmt.Sprintf("Outcome: %s (%s)", action.ActionType.Label(), action.Status.Label())
	}
	parts = append(parts, outcome)
	if snapshot := templateSnapshotResponse(item.TemplateSnapshotJSON); snapshot != nil && snapshot.Template.Appealable {
		parts = append(parts, "This case can be appealed from your Quack dashboard.")
	}
	if settings != nil && strings.TrimSpace(settings.NotificationFooter) != "" {
		parts = append(parts, truncateRunes(strings.TrimSpace(settings.NotificationFooter), 150))
	}
	return truncateRunes(strings.Join(parts, "\n"), 2000)
}

// redactDiscordError converts adapter failures to safe durable notification diagnostics.
func redactDiscordError(err error) string {
	if err == nil {
		return ""
	}
	var discordErr actionmods.DiscordError
	if errors.As(err, &discordErr) {
		return discordErr.Error()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "Discord request timed out"
	}
	return "Discord request failed"
}
