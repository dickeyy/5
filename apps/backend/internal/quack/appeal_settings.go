package quack

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// DefaultAppealQuestions returns Quack's stable, simple default form.
func DefaultAppealQuestions() []model.AppealQuestion {
	return []model.AppealQuestion{
		{ID: "reason", Prompt: "Why should this case be reconsidered?", Type: model.AppealQuestionLongText, Required: true, Position: 0},
		{ID: "context", Prompt: "Is there any additional context staff should review?", Type: model.AppealQuestionLongText, Required: false, Position: 1},
	}
}

// GetSettings returns the configured or default appeal form for one guild.
func (s *AppealService) GetSettings(ctx context.Context, guildID string) (*AppealSettingsResponse, error) {
	if s == nil || s.store == nil || strings.TrimSpace(guildID) == "" {
		return nil, appealValidation("guild is required")
	}
	settings, err := s.store.GetGuildAppealSettings(ctx, strings.TrimSpace(guildID))
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return &AppealSettingsResponse{GuildID: guildID, Questions: DefaultAppealQuestions(), Default: true}, nil
	}
	questions, err := decodeQuestions(settings.QuestionsJSON)
	if err != nil {
		return nil, err
	}
	return &AppealSettingsResponse{GuildID: settings.GuildID, Questions: questions}, nil
}

// UpdateSettings validates and replaces only the form snapshotted by future appeals.
func (s *AppealService) UpdateSettings(ctx context.Context, guildContext *GuildStaffContext, questions []model.AppealQuestion) (*AppealSettingsResponse, error) {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil || !guildContext.Can(model.PermissionActionGuildSettingsWrite) {
		return nil, ErrAppealPermissionDenied
	}
	normalized, err := validateQuestions(questions)
	if err != nil {
		return nil, err
	}
	body, _ := json.Marshal(normalized)
	settings, err := s.store.UpdateGuildAppealSettings(ctx, model.UpdateGuildAppealSettingsParams{
		Settings: model.GuildAppealSettings{GuildID: guildContext.Guild.ID, QuestionsJSON: string(body), UpdatedByDiscordUserID: guildContext.Staff.DiscordUserID},
		Audit:    appealAudit(ctx, guildContext.Guild.ID, guildContext.Staff.DiscordUserID, guildContext.PermissionBits, "appeal.settings.update", "guild_appeal_settings", "", model.AuditResultSuccess),
	})
	if err != nil {
		return nil, err
	}
	return &AppealSettingsResponse{GuildID: settings.GuildID, Questions: normalized}, nil
}
