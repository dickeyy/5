package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/discordbot/interactions"
	"github.com/quackdiscord/bot/internal/discordbot/ui"
	"github.com/quackdiscord/bot/internal/discordbot/ui/views"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// RegisterCaseComponents installs QP-E case browsing and recovery handlers.
// QI-2 installs the separate QP-D appeal registrar alongside this registrar.
func RegisterCaseComponents(registry *interactions.ComponentRegistry) error {
	components := map[string]ui.Handler{
		"list_prev": pageCases(-1, false), "list_next": pageCases(1, false),
		"user_prev": pageCases(-1, true), "user_next": pageCases(1, true),
		"failures_prev": pageFailures(-1), "failures_next": pageFailures(1),
		"retry": handleRetryComponent, "dismiss": handleDismissComponent,
		"void": handleVoidComponent, "reverse": handleReverseComponent,
		"message_template": handleMessageTemplateComponent,
		"context_next":     handleContextNextComponent,
	}
	for action, handler := range components {
		if err := registry.RegisterComponent("case", action, handler); err != nil {
			return err
		}
	}
	for action, handler := range map[string]ui.Handler{"void_submit": handleVoidModal, "reverse_submit": handleReverseModal, "context_submit": handleContextModal} {
		if err := registry.RegisterModal("case", action, handler); err != nil {
			return err
		}
	}
	return nil
}

type caseContextDraft struct {
	Token                   string
	ActorDiscordUserID      string
	GuildID                 string
	ContextChannelDiscordID string
	ContextMessageDiscordID string
	TargetDiscordUserID     string
	Template                quack.TemplateResponse
	Values                  map[string]json.RawMessage
	EvidenceLinks           []string
	Page                    int
	ExpiresAt               time.Time
}

type caseContextDraftStore struct {
	mu     sync.Mutex
	drafts map[string]caseContextDraft
}

var caseContextDrafts = &caseContextDraftStore{drafts: map[string]caseContextDraft{}}

func (s *caseContextDraftStore) put(draft caseContextDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for token, existing := range s.drafts {
		if !existing.ExpiresAt.After(now) {
			delete(s.drafts, token)
		}
	}
	s.drafts[draft.Token] = draft
}

func (s *caseContextDraftStore) get(token string) (caseContextDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	draft, ok := s.drafts[token]
	if !ok || !draft.ExpiresAt.After(time.Now().UTC()) {
		delete(s.drafts, token)
		return caseContextDraft{}, false
	}
	values := make(map[string]json.RawMessage, len(draft.Values))
	for key, value := range draft.Values {
		values[key] = append(json.RawMessage(nil), value...)
	}
	draft.Values = values
	draft.EvidenceLinks = append([]string(nil), draft.EvidenceLinks...)
	return draft, true
}

func (s *caseContextDraftStore) save(draft caseContextDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drafts[draft.Token] = draft
}

func (s *caseContextDraftStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.drafts, token)
}

func pageCases(delta int, user bool) ui.Handler {
	return func(ctx ui.Context) ui.HandlerResult {
		parsed, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
		if err != nil {
			return ui.Immediate(ui.Error("That case page is no longer available."))
		}
		parts := strings.SplitN(parsed.Payload, "|", 2)
		page, _ := strconv.Atoi(parts[0])
		page += delta
		if page < 1 {
			page = 1
		}
		targetID := ""
		if len(parts) == 2 {
			targetID = parts[1]
		}
		return ui.Async(ui.DeferUpdate(), func(taskCtx context.Context, responder ui.Responder) error {
			guildContext, resolveErr := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
			if resolveErr != nil {
				_, editErr := responder.UpdateMessage(ui.ErrorEdit(caseCommandErrorMessage(resolveErr)))
				return editErr
			}
			input := quack.CaseListInput{Limit: "10", Offset: strconv.Itoa((page - 1) * 10)}
			var list *quack.CaseListResponse
			if user {
				profile, listErr := ctx.Services.Cases.UserHistory(taskCtx, guildContext, targetID, input)
				if listErr != nil {
					return listErr
				}
				list = &quack.CaseListResponse{Cases: profile.Cases, Total: profile.Total, Limit: profile.Limit, Offset: profile.Offset}
			} else {
				list, err = ctx.Services.Cases.List(taskCtx, guildContext, input)
				if err != nil {
					return err
				}
			}
			_, editErr := responder.UpdateMessage(ui.EditMessage(views.CaseListMessage(list, page, targetID)))
			return editErr
		})
	}
}

func pageFailures(delta int) ui.Handler {
	return func(ctx ui.Context) ui.HandlerResult {
		parsed, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
		if err != nil {
			return ui.Immediate(ui.Error("That failure page is no longer available."))
		}
		page, _ := strconv.Atoi(parsed.Payload)
		page += delta
		if page < 1 {
			page = 1
		}
		return ui.Async(ui.DeferUpdate(), func(taskCtx context.Context, responder ui.Responder) error {
			guildContext, resolveErr := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
			if resolveErr != nil {
				return resolveErr
			}
			result, listErr := ctx.Services.Actions.ListFailures(taskCtx, guildContext, 10, (page-1)*10)
			if listErr != nil {
				return listErr
			}
			_, editErr := responder.UpdateMessage(ui.EditMessage(views.FailedActionMessage(result, page)))
			return editErr
		})
	}
}

func handleRetryComponent(ctx ui.Context) ui.HandlerResult {
	return actionControlComponent(ctx, "retry")
}

func handleDismissComponent(ctx ui.Context) ui.HandlerResult {
	return actionControlComponent(ctx, "dismiss")
}

func actionControlComponent(ctx ui.Context, operation string) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
	if err != nil {
		return ui.Immediate(ui.Error("That action control is invalid."))
	}
	return ui.Async(ui.DeferUpdate(), func(taskCtx context.Context, responder ui.Responder) error {
		guildContext, resolveErr := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
		if resolveErr != nil {
			return resolveErr
		}
		var controlErr error
		if operation == "retry" {
			_, controlErr = ctx.Services.Actions.Retry(taskCtx, guildContext, parsed.Payload)
		} else {
			_, controlErr = ctx.Services.Actions.Dismiss(taskCtx, guildContext, parsed.Payload)
		}
		if controlErr != nil {
			return controlErr
		}
		result, listErr := ctx.Services.Actions.ListFailures(taskCtx, guildContext, 10, 0)
		if listErr != nil {
			return listErr
		}
		_, editErr := responder.UpdateMessage(ui.EditMessage(views.FailedActionMessage(result, 1)))
		return editErr
	})
}

func handleVoidComponent(ctx ui.Context) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
	if err != nil || strings.TrimSpace(parsed.Payload) == "" {
		return ui.Immediate(ui.Error("That case control is invalid."))
	}
	customID := ui.MustCustomID(ui.CustomID{Namespace: "case", Action: "void_submit", Version: "v1", Payload: parsed.Payload})
	return ui.Immediate(ui.Modal("Void case", customID, []discordgo.MessageComponent{ui.Row(discordgo.TextInput{CustomID: "reason", Label: "Required correction reason", Style: discordgo.TextInputParagraph, Required: true, MinLength: 3, MaxLength: 500})}))
}

func handleVoidModal(ctx ui.Context) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.ModalSubmitData().CustomID)
	if err != nil {
		return ui.Immediate(ui.Error("That case control is invalid."))
	}
	reason := modalTextValue(ctx.Interaction.ModalSubmitData(), "reason")
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		guildContext, resolveErr := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
		if resolveErr != nil {
			return resolveErr
		}
		item, voidErr := ctx.Services.Cases.Void(taskCtx, guildContext, parsed.Payload, reason, nil)
		if voidErr != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(voidErr)))
			return editErr
		}
		_, editErr := responder.EditOriginal(ui.EditMessage(ui.EmbedMessage(ui.SuccessEmbed("Case voided", fmt.Sprintf("Case #%d remains in history and no longer contributes to escalation.", item.CaseNumber)), true)))
		return editErr
	})
}

func handleReverseComponent(ctx ui.Context) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
	if err != nil || len(strings.Split(parsed.Payload, "|")) != 3 {
		return ui.Immediate(ui.Error("That reversal control is invalid."))
	}
	customID := ui.MustCustomID(ui.CustomID{Namespace: "case", Action: "reverse_submit", Version: "v1", Payload: parsed.Payload})
	return ui.Immediate(ui.Modal("Confirm reversal", customID, []discordgo.MessageComponent{ui.Row(discordgo.TextInput{CustomID: "confirm", Label: "Type REVERSE to confirm", Style: discordgo.TextInputShort, Required: true, MinLength: 7, MaxLength: 7})}))
}

func handleReverseModal(ctx ui.Context) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.ModalSubmitData().CustomID)
	parts := strings.Split(parsed.Payload, "|")
	if err != nil || len(parts) != 3 || modalTextValue(ctx.Interaction.ModalSubmitData(), "confirm") != "REVERSE" {
		return ui.Immediate(ui.Error("Reversal confirmation did not match."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		guildContext, resolveErr := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
		if resolveErr != nil {
			return resolveErr
		}
		_, reverseErr := ctx.Services.Actions.Reverse(taskCtx, guildContext, parts[0], parts[1], model.ActionType(parts[2]))
		if reverseErr != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(reverseErr)))
			return editErr
		}
		_, editErr := responder.EditOriginal(ui.EditMessage(ui.EmbedMessage(ui.SuccessEmbed("Reversal queued", "The original action remains visible in case history."), true)))
		return editErr
	})
}

func handleContextModal(ctx ui.Context) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.ModalSubmitData().CustomID)
	draft, ok := caseContextDrafts.get(parsed.Payload)
	if err != nil || !ok || !caseDraftMatchesInteraction(draft, ctx.Interaction) {
		return ui.Immediate(ui.Error("That case context form is invalid."))
	}
	fields := contextDraftPageFields(draft)
	values, evidence, valuesErr := contextValuesFromModalFields(ctx.Interaction.ModalSubmitData(), fields)
	if valuesErr != nil {
		return ui.Immediate(ui.Error(valuesErr.Error()))
	}
	for _, value := range values {
		draft.Values[value.Key] = value.Value
	}
	draft.EvidenceLinks = appendUniqueStrings(draft.EvidenceLinks, evidence...)
	draft.Page++
	if draft.Page*5 < len(draft.Template.ContextFields) {
		caseContextDrafts.save(draft)
		continueID := ui.MustCustomID(ui.CustomID{Namespace: "case", Action: "context_next", Version: "v1", Payload: draft.Token})
		return ui.Immediate(ui.Ephemeral(ui.Message{Content: fmt.Sprintf("Saved context page %d. Continue to page %d.", draft.Page, draft.Page+1), Components: []discordgo.MessageComponent{ui.Row(ui.Button(continueID, "Continue context", discordgo.PrimaryButton, false))}, Ephemeral: true}))
	}
	caseContextDrafts.delete(draft.Token)
	orderedValues := make([]quack.CaseContextValueInput, 0, len(draft.Values))
	for _, field := range draft.Template.ContextFields {
		if value, exists := draft.Values[field.Key]; exists {
			orderedValues = append(orderedValues, quack.CaseContextValueInput{Key: field.Key, Value: value})
		}
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		guildContext, resolveErr := resolveInteractionGuildContext(taskCtx, ctx.Services, ctx.Interaction)
		if resolveErr != nil {
			return resolveErr
		}
		_, template, resolveTemplateErr := resolveTemplate(taskCtx, ctx.Services, guildContext, draft.Template.ID)
		if resolveTemplateErr != nil || template == nil {
			return quack.ErrCaseTemplateNotAvailable
		}
		created, createErr := ctx.Services.Cases.Create(taskCtx, guildContext, quack.CaseInput{TemplateID: template.ID, TargetDiscordUserID: draft.TargetDiscordUserID, Source: model.CaseSourceDiscord, ContextChannelDiscordID: draft.ContextChannelDiscordID, ContextMessageDiscordID: draft.ContextMessageDiscordID, ContextValues: orderedValues, EvidenceLinks: draft.EvidenceLinks, IdempotencyKey: draft.Token})
		if createErr != nil {
			_, editErr := responder.EditOriginal(ui.ErrorEdit(caseCommandErrorMessage(createErr)))
			return editErr
		}
		message, followErr := responder.Followup(views.CaseCreatedMessage(views.CaseCreated{Case: created, Template: template}))
		if followErr == nil {
			_ = responder.DeleteOriginal()
			if message != nil {
				updatePublicCaseResult(responder, ctx.Services, created, message.ID, template)
			}
		}
		return followErr
	})
}

func handleContextNextComponent(ctx ui.Context) ui.HandlerResult {
	parsed, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
	draft, ok := caseContextDrafts.get(parsed.Payload)
	if err != nil || !ok || !caseDraftMatchesInteraction(draft, ctx.Interaction) {
		return ui.Immediate(ui.Error("That case context form expired."))
	}
	modal, modalErr := contextDraftModal(draft)
	if modalErr != nil {
		return ui.Immediate(ui.Error("That case context form is unavailable."))
	}
	return ui.Immediate(modal)
}

func handleMessageTemplateComponent(ctx ui.Context) ui.HandlerResult {
	data := ctx.Interaction.MessageComponentData()
	parsed, err := ui.DecodeCustomID(data.CustomID)
	parts := strings.Split(parsed.Payload, "|")
	if err != nil || len(parts) != 3 || len(data.Values) != 1 {
		return ui.Immediate(ui.Error("That message case flow is invalid."))
	}
	guildContext, resolveErr := resolveInteractionGuildContext(ctx.Context, ctx.Services, ctx.Interaction)
	if resolveErr != nil {
		return ui.Immediate(ui.Error(caseCommandErrorMessage(resolveErr)))
	}
	_, template, templateErr := resolveTemplate(ctx.Context, ctx.Services, guildContext, data.Values[0])
	if templateErr != nil || template == nil {
		return ui.Immediate(ui.Error("That case template is not available."))
	}
	link := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ctx.Interaction.GuildID, parts[1], parts[2])
	values := messageLinkContext(template, link)
	if len(values) != len(template.ContextFields) {
		modal, modalErr := contextModalWithPrefill(ctx.Interaction, template, parts[0], link, parts[1], parts[2], values)
		if modalErr != nil {
			return ui.Immediate(ui.Error("That case context form is unavailable."))
		}
		return ui.Immediate(modal)
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		created, createErr := ctx.Services.Cases.Create(taskCtx, guildContext, quack.CaseInput{TemplateID: template.ID, TargetDiscordUserID: parts[0], Source: model.CaseSourceDiscord, ContextChannelDiscordID: parts[1], ContextMessageDiscordID: parts[2], ContextValues: values, EvidenceLinks: []string{link}, IdempotencyKey: ctx.Interaction.ID})
		if createErr != nil {
			return createErr
		}
		message, followErr := responder.Followup(views.CaseCreatedMessage(views.CaseCreated{Case: created, Template: template}))
		if followErr == nil {
			_ = responder.DeleteOriginal()
			if message != nil {
				updatePublicCaseResult(responder, ctx.Services, created, message.ID, template)
			}
		}
		return followErr
	})
}

func contextValuesFromModalFields(data discordgo.ModalSubmitInteractionData, fields []quack.TemplateContextFieldResponse) ([]quack.CaseContextValueInput, []string, error) {
	values := make([]quack.CaseContextValueInput, 0, len(fields))
	evidence := []string{}
	for _, field := range fields {
		value := strings.TrimSpace(modalTextValue(data, "context_"+field.Key))
		if value == "" {
			if field.Required {
				return nil, nil, fmt.Errorf("%s is required", field.Label)
			}
			continue
		}
		var raw json.RawMessage
		switch field.FieldType {
		case model.ContextFieldBoolean:
			parsed, err := strconv.ParseBool(strings.ToLower(value))
			if err != nil {
				return nil, nil, fmt.Errorf("%s must be true or false", field.Label)
			}
			raw, _ = json.Marshal(parsed)
		case model.ContextFieldNumber:
			if _, err := strconv.ParseFloat(value, 64); err != nil {
				return nil, nil, fmt.Errorf("%s must be a number", field.Label)
			}
			raw = json.RawMessage(value)
		default:
			raw, _ = json.Marshal(value)
		}
		if field.FieldType == model.ContextFieldMessageLink {
			evidence = append(evidence, value)
		}
		values = append(values, quack.CaseContextValueInput{Key: field.Key, Value: raw})
	}
	return values, evidence, nil
}

func messageLinkContext(template *quack.TemplateResponse, link string) []quack.CaseContextValueInput {
	values := []quack.CaseContextValueInput{}
	for _, field := range template.ContextFields {
		if field.FieldType != model.ContextFieldMessageLink {
			continue
		}
		raw, _ := json.Marshal(link)
		values = append(values, quack.CaseContextValueInput{Key: field.Key, Value: raw})
	}
	return values
}

func modalTextValue(data discordgo.ModalSubmitInteractionData, customID string) string {
	for _, component := range data.Components {
		row, ok := component.(*discordgo.ActionsRow)
		if !ok {
			if value, valueOK := component.(discordgo.ActionsRow); valueOK {
				row = &value
			} else {
				continue
			}
		}
		for _, child := range row.Components {
			input, ok := child.(*discordgo.TextInput)
			if ok && input.CustomID == customID {
				return input.Value
			}
			if input, ok := child.(discordgo.TextInput); ok && input.CustomID == customID {
				return input.Value
			}
		}
	}
	return ""
}

func contextModal(interaction *discordgo.InteractionCreate, template *quack.TemplateResponse, targetID, evidenceLink string) (*discordgo.InteractionResponse, error) {
	return contextModalWithPrefill(interaction, template, targetID, evidenceLink, interaction.ChannelID, "", nil)
}

func contextModalWithPrefill(interaction *discordgo.InteractionCreate, template *quack.TemplateResponse, targetID, evidenceLink, channelID, messageID string, prefilled []quack.CaseContextValueInput) (*discordgo.InteractionResponse, error) {
	if interaction == nil || template == nil || len(template.ContextFields) == 0 || strings.TrimSpace(interaction.ID) == "" {
		return nil, errors.New("template context is empty")
	}
	actorID, _, _ := interactionMemberFields(interaction)
	values := map[string]json.RawMessage{}
	for _, value := range prefilled {
		values[value.Key] = value.Value
	}
	draft := caseContextDraft{Token: interaction.ID, ActorDiscordUserID: actorID, GuildID: interaction.GuildID, ContextChannelDiscordID: channelID, ContextMessageDiscordID: messageID, TargetDiscordUserID: targetID, Template: *template, Values: values, EvidenceLinks: appendUniqueStrings(nil, evidenceLink), ExpiresAt: time.Now().UTC().Add(15 * time.Minute)}
	caseContextDrafts.put(draft)
	return contextDraftModal(draft)
}

func contextDraftModal(draft caseContextDraft) (*discordgo.InteractionResponse, error) {
	customID, err := ui.EncodeCustomID(ui.CustomID{Namespace: "case", Action: "context_submit", Version: "v1", Payload: draft.Token})
	if err != nil {
		return nil, err
	}
	fields := contextDraftPageFields(draft)
	rows := make([]discordgo.MessageComponent, 0, len(fields))
	for _, field := range fields {
		style := discordgo.TextInputShort
		maxLength := 1000
		if field.FieldType == model.ContextFieldLongText {
			style = discordgo.TextInputParagraph
			maxLength = 4000
		}
		placeholder := "Enter a value"
		if field.FieldType == model.ContextFieldBoolean {
			placeholder = "true or false"
		}
		value := ""
		if raw, exists := draft.Values[field.Key]; exists {
			_ = json.Unmarshal(raw, &value)
		}
		rows = append(rows, ui.Row(discordgo.TextInput{CustomID: "context_" + field.Key, Label: field.Label, Style: style, Required: field.Required, MaxLength: maxLength, Placeholder: placeholder, Value: value}))
	}
	return ui.Modal(fmt.Sprintf("Case context (%d/%d)", draft.Page+1, (len(draft.Template.ContextFields)+4)/5), customID, rows), nil
}

func contextDraftPageFields(draft caseContextDraft) []quack.TemplateContextFieldResponse {
	start := draft.Page * 5
	if start >= len(draft.Template.ContextFields) {
		return nil
	}
	end := start + 5
	if end > len(draft.Template.ContextFields) {
		end = len(draft.Template.ContextFields)
	}
	return draft.Template.ContextFields[start:end]
}

func caseDraftMatchesInteraction(draft caseContextDraft, interaction *discordgo.InteractionCreate) bool {
	if interaction == nil || draft.GuildID != interaction.GuildID {
		return false
	}
	actorID, _, _ := interactionMemberFields(interaction)
	return actorID != "" && actorID == draft.ActorDiscordUserID
}

func appendUniqueStrings(values []string, additions ...string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = struct{}{}
		}
	}
	for _, value := range additions {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		values = append(values, value)
		seen[value] = struct{}{}
	}
	return values
}
