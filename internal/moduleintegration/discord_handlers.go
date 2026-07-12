package moduleintegration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/discordbot/interactions"
	"github.com/quackdiscord/bot/internal/discordbot/ui"
	"github.com/quackdiscord/bot/internal/modules"
	"github.com/quackdiscord/bot/internal/modules/generallogging"
	"github.com/quackdiscord/bot/internal/modules/honeypot"
	"github.com/quackdiscord/bot/internal/modules/tickets"
	"github.com/rs/zerolog/log"
)

// RegisterComponents installs ticket buttons and the reply modal into the
// process's single interaction dispatcher.
func (r *Runtime) RegisterComponents(registry *interactions.ComponentRegistry) error {
	if r == nil || r.TicketDiscord == nil || r.Tickets == nil {
		return errors.New("ticket Discord runtime is not configured")
	}
	handlers := tickets.ComponentHandlers{
		Open: r.openTicketComponent, Queue: r.ticketQueueComponent,
		View: r.viewTicketComponent, Reply: r.replyTicketComponent,
		Close: r.closeTicketComponent,
	}
	if err := tickets.RegisterComponents(registry, handlers); err != nil {
		return err
	}
	if err := registry.RegisterComponent("ticket", "repair", r.repairTicketComponent); err != nil {
		return err
	}
	return registry.RegisterModal("ticket", "reply-submit", r.submitTicketReplyModal)
}

// RegisterGatewayHandlers subscribes optional modules to gateway events without
// placing their failures on the moderation action queue.
func (r *Runtime) RegisterGatewayHandlers(session *discordgo.Session) error {
	if r == nil || r.LoggingQueue == nil || r.HoneypotRuntime == nil || r.HoneypotDiscord == nil || session == nil {
		return errors.New("optional module gateway runtime is not configured")
	}
	session.AddHandler(r.onMessageCreate)
	session.AddHandler(r.onMessageUpdate)
	session.AddHandler(r.onMessageDelete)
	session.AddHandler(r.onMessageDeleteBulk)
	session.AddHandler(r.onGuildMemberAdd)
	session.AddHandler(r.onGuildMemberRemove)
	session.AddHandler(r.onGuildBanAdd)
	session.AddHandler(r.onGuildBanRemove)
	session.AddHandler(r.onGuildUpdate)
	session.AddHandler(r.onGuildDelete)
	session.AddHandler(r.onChannelCreate)
	session.AddHandler(r.onChannelUpdate)
	session.AddHandler(r.onChannelDelete)
	return nil
}

// ticketActor resolves the current Discord member into module authority.
func (r *Runtime) ticketActor(ctx ui.Context) (tickets.Actor, error) {
	if ctx.Interaction == nil || ctx.Interaction.GuildID == "" {
		return tickets.Actor{}, errors.New("ticket interactions require a guild")
	}
	userID := interactionUserID(ctx.Interaction)
	if userID == "" {
		return tickets.Actor{}, errors.New("ticket interaction user is unavailable")
	}
	guildID, err := (guildResolver{db: r.db}).internalID(ctx, ctx.Interaction.GuildID)
	if err != nil {
		return tickets.Actor{}, err
	}
	permissions, err := ctx.Session.UserChannelPermissions(userID, ctx.Interaction.ChannelID)
	if err != nil {
		return tickets.Actor{}, err
	}
	administrator := permissions&discordgo.PermissionAdministrator != 0
	return tickets.Actor{
		GuildID: guildID, DiscordUserID: userID,
		CanManage:   administrator || permissions&discordgo.PermissionManageServer != 0,
		CanModerate: administrator || permissions&discordgo.PermissionModerateMembers != 0,
	}, nil
}

// openTicketComponent acknowledges quickly, then provisions the private ticket.
func (r *Runtime) openTicketComponent(ctx ui.Context) ui.HandlerResult {
	actor, err := r.ticketActor(ctx)
	if err != nil {
		return ui.Immediate(ui.Error("Quack could not verify your ticket access."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		ticket, err := r.TicketDiscord.Open(taskCtx, actor)
		if err != nil {
			_, _ = responder.EditOriginal(ui.ErrorEdit(ticketErrorMessage(err)))
			return nil
		}
		message := ui.Content("Ticket opened: <#"+ticket.ThreadDiscordChannelID+">", true)
		message.Components = ticketControls(ticket.ID, actor.CanManage)
		_, err = responder.EditOriginal(ui.EditMessage(message))
		return err
	})
}

// ticketQueueComponent returns a bounded staff queue without transcript content.
func (r *Runtime) ticketQueueComponent(ctx ui.Context) ui.HandlerResult {
	actor, err := r.ticketActor(ctx)
	if err != nil || !actor.CanModerate {
		return ui.Immediate(ui.Error("You do not have access to the ticket queue."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		queue, err := r.Tickets.Queue(taskCtx, actor, tickets.StatusOpen, 25)
		if err != nil {
			_, _ = responder.EditOriginal(ui.ErrorEdit(ticketErrorMessage(err)))
			return nil
		}
		lines := []string{"Open tickets:"}
		for _, ticket := range queue {
			lines = append(lines, fmt.Sprintf("• `%s` — <#%s>", ticket.ID, ticket.ThreadDiscordChannelID))
		}
		if len(queue) == 0 {
			lines = append(lines, "No open tickets.")
		}
		_, err = responder.EditOriginal(ui.EditMessage(ui.Content(strings.Join(lines, "\n"), true)))
		return err
	})
}

// viewTicketComponent returns private ticket state and its immutable timeline.
func (r *Runtime) viewTicketComponent(ctx ui.Context) ui.HandlerResult {
	actor, ticketID, err := r.ticketComponentInput(ctx)
	if err != nil {
		return ui.Immediate(ui.Error("That ticket is unavailable."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		ticket, events, err := r.Tickets.Detail(taskCtx, actor, ticketID)
		if err != nil {
			_, _ = responder.EditOriginal(ui.ErrorEdit(ticketErrorMessage(err)))
			return nil
		}
		lines := []string{fmt.Sprintf("Ticket `%s` is **%s**.", ticket.ID, ticket.Status)}
		for _, event := range events {
			lines = append(lines, fmt.Sprintf("• %s — %s", event.Type, ui.TruncateRunes(event.Body, 240)))
		}
		message := ui.Content(strings.Join(lines, "\n"), true)
		message.Components = ticketControls(ticket.ID, actor.CanManage)
		_, err = responder.EditOriginal(ui.EditMessage(message))
		return err
	})
}

// repairTicketComponent restores the configured private ACL for current managers.
func (r *Runtime) repairTicketComponent(ctx ui.Context) ui.HandlerResult {
	actor, ticketID, err := r.ticketComponentInput(ctx)
	if err != nil || !actor.CanManage {
		return ui.Immediate(ui.Error("You do not have permission to repair that ticket."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		if err := r.TicketDiscord.RepairPermissions(taskCtx, actor, ticketID); err != nil {
			_, _ = responder.EditOriginal(ui.ErrorEdit(ticketErrorMessage(err)))
			return nil
		}
		_, err := responder.EditOriginal(ui.EditMessage(ui.Content("Ticket permissions repaired.", true)))
		return err
	})
}

// ticketControls adds manager-only repair behavior to the module-owned controls.
func ticketControls(ticketID string, includeRepair bool) []discordgo.MessageComponent {
	components := tickets.TicketComponents(ticketID)
	if !includeRepair || len(components) == 0 {
		return components
	}
	row, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		return components
	}
	repairID := ui.MustCustomID(ui.CustomID{Namespace: "ticket", Action: "repair", Version: "v1", Payload: ticketID})
	row.Components = append(row.Components, ui.Button(repairID, "Repair permissions", discordgo.SecondaryButton, false))
	components[0] = row
	return components
}

// replyTicketComponent opens a modal so reply content never enters a custom ID.
func (r *Runtime) replyTicketComponent(ctx ui.Context) ui.HandlerResult {
	_, ticketID, err := r.ticketComponentInput(ctx)
	if err != nil {
		return ui.Immediate(ui.Error("That ticket is unavailable."))
	}
	customID := ui.MustCustomID(ui.CustomID{Namespace: "ticket", Action: "reply-submit", Version: "v1", Payload: ticketID})
	components := []discordgo.MessageComponent{discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.TextInput{CustomID: "body", Label: "Reply", Style: discordgo.TextInputParagraph, Required: true, MinLength: 1, MaxLength: 4000},
	}}}
	return ui.Immediate(ui.Modal("Reply to ticket", customID, components))
}

// submitTicketReplyModal validates the modal and sends through the private adapter.
func (r *Runtime) submitTicketReplyModal(ctx ui.Context) ui.HandlerResult {
	actor, err := r.ticketActor(ctx)
	if err != nil {
		return ui.Immediate(ui.Error("Quack could not verify your ticket access."))
	}
	data := ctx.Interaction.ModalSubmitData()
	customID, err := ui.DecodeCustomID(data.CustomID)
	if err != nil || customID.Payload == "" {
		return ui.Immediate(ui.Error("That ticket reply is invalid."))
	}
	body := modalText(data.Components, "body")
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		if err := r.TicketDiscord.Reply(taskCtx, actor, customID.Payload, body); err != nil {
			_, _ = responder.EditOriginal(ui.ErrorEdit(ticketErrorMessage(err)))
			return nil
		}
		_, err := responder.EditOriginal(ui.EditMessage(ui.Content("Reply sent.", true)))
		return err
	})
}

// closeTicketComponent captures the transcript and resolves the ticket.
func (r *Runtime) closeTicketComponent(ctx ui.Context) ui.HandlerResult {
	actor, ticketID, err := r.ticketComponentInput(ctx)
	if err != nil || !actor.CanModerate {
		return ui.Immediate(ui.Error("You do not have permission to close that ticket."))
	}
	return ui.Async(ui.DeferEphemeral(), func(taskCtx context.Context, responder ui.Responder) error {
		if _, err := r.TicketDiscord.Close(taskCtx, actor, ticketID); err != nil {
			_, _ = responder.EditOriginal(ui.ErrorEdit(ticketErrorMessage(err)))
			return nil
		}
		_, err := responder.EditOriginal(ui.EditMessage(ui.Content("Ticket closed and transcript captured.", true)))
		return err
	})
}

// ticketComponentInput extracts the actor and opaque ticket identity.
func (r *Runtime) ticketComponentInput(ctx ui.Context) (tickets.Actor, string, error) {
	actor, err := r.ticketActor(ctx)
	if err != nil {
		return tickets.Actor{}, "", err
	}
	customID, err := ui.DecodeCustomID(ctx.Interaction.MessageComponentData().CustomID)
	if err != nil || customID.Payload == "" {
		return tickets.Actor{}, "", errors.New("ticket id is required")
	}
	return actor, customID.Payload, nil
}

// interactionUserID normalizes guild and direct interaction identity fields.
func interactionUserID(interaction *discordgo.InteractionCreate) string {
	if interaction.Member != nil && interaction.Member.User != nil {
		return interaction.Member.User.ID
	}
	if interaction.User != nil {
		return interaction.User.ID
	}
	return ""
}

// modalText extracts one expected text input from Discord's component tree.
func modalText(components []discordgo.MessageComponent, customID string) string {
	for _, component := range components {
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
				return strings.TrimSpace(input.Value)
			}
			if value, valueOK := child.(discordgo.TextInput); valueOK && value.CustomID == customID {
				return strings.TrimSpace(value.Value)
			}
		}
	}
	return ""
}

// ticketErrorMessage maps internal classifications to safe Discord copy.
func ticketErrorMessage(err error) string {
	switch {
	case errors.Is(err, tickets.ErrDisabled):
		return "Tickets are not enabled for this server."
	case errors.Is(err, tickets.ErrPermissionDenied):
		return "You do not have permission to use that ticket."
	case errors.Is(err, tickets.ErrDuplicateOpen):
		return "You already have an open ticket."
	case errors.Is(err, tickets.ErrRateLimited):
		return "You have reached this server's ticket limit."
	case errors.Is(err, tickets.ErrNotFound):
		return "That ticket was not found."
	default:
		return "Quack could not complete that ticket operation."
	}
}

// submit enqueues general logging without blocking the gateway or moderation.
func (r *Runtime) submit(event generallogging.Event) {
	if r == nil || r.LoggingQueue == nil {
		return
	}
	if err := r.LoggingQueue.Submit(event); err != nil && !errors.Is(err, generallogging.ErrQueueFull) {
		log.Error().Err(err).Msg("Failed to queue general logging event")
	}
}

// internalGuildID resolves active guilds and suppresses events for unknown guilds.
func (r *Runtime) internalGuildID(discordGuildID string) (string, bool) {
	id, err := (guildResolver{db: r.db}).internalID(context.Background(), discordGuildID)
	return id, err == nil
}

// onMessageCreate retains bounded context only when logging is enabled.
func (r *Runtime) onMessageCreate(_ *discordgo.Session, event *discordgo.MessageCreate) {
	r.submitHoneypotMessage(event)
	if event == nil || event.Message == nil || event.GuildID == "" || (event.Author != nil && event.Author.Bot) {
		return
	}
	guildID, ok := r.internalGuildID(event.GuildID)
	if !ok {
		return
	}
	_ = r.Logging.CacheMessage(context.Background(), cachedMessage(guildID, event.Message))
}

// submitHoneypotMessage performs live member/permission projection only for an
// enabled guild, then submits to the module's isolated bounded runtime.
func (r *Runtime) submitHoneypotMessage(event *discordgo.MessageCreate) {
	if r == nil || r.registry == nil || r.session == nil || r.HoneypotRuntime == nil || event == nil || event.Message == nil || event.GuildID == "" {
		return
	}
	ctx := context.Background()
	guildID, ok := r.internalGuildID(event.GuildID)
	if !ok {
		return
	}
	configuration, err := r.registry.Configuration(ctx, guildID, modules.Honeypots)
	if err != nil || configuration == nil || !configuration.Enabled {
		return
	}
	channel, err := r.session.Channel(event.ChannelID)
	if err != nil || channel == nil || channel.GuildID != event.GuildID || event.Author == nil {
		return
	}
	guild, err := r.session.Guild(event.GuildID)
	if err != nil || guild == nil {
		return
	}
	member, err := r.session.GuildMember(event.GuildID, event.Author.ID)
	if err != nil || member == nil {
		return
	}
	message, err := projectHoneypotMessage(event, guild, channel, member, currentBotID(r.session))
	if err != nil {
		return
	}
	if err := r.HoneypotRuntime.Submit(message); err != nil && !errors.Is(err, honeypot.ErrQueueFull) {
		log.Error().Err(err).Str("guild_id", event.GuildID).Msg("Failed to queue honeypot event")
	}
}

// onMessageUpdate queues an edit and refreshes bounded cache context.
func (r *Runtime) onMessageUpdate(_ *discordgo.Session, event *discordgo.MessageUpdate) {
	if event == nil || event.Message == nil || event.GuildID == "" {
		return
	}
	guildID, ok := r.internalGuildID(event.GuildID)
	if !ok {
		return
	}
	before := ""
	if event.BeforeUpdate != nil {
		before = event.BeforeUpdate.Content
	}
	r.submit(messageEvent(guildID, generallogging.MessageEdit, event.Message, before, event.Content))
	_ = r.Logging.CacheMessage(context.Background(), cachedMessage(guildID, event.Message))
}

// onMessageDelete queues a cache-enriched deletion event.
func (r *Runtime) onMessageDelete(_ *discordgo.Session, event *discordgo.MessageDelete) {
	if event == nil || event.Message == nil || event.GuildID == "" {
		return
	}
	guildID, ok := r.internalGuildID(event.GuildID)
	if ok {
		r.submit(messageEvent(guildID, generallogging.MessageDelete, event.Message, "", ""))
	}
}

// onMessageDeleteBulk queues bounded cache-aware bulk work.
func (r *Runtime) onMessageDeleteBulk(_ *discordgo.Session, event *discordgo.MessageDeleteBulk) {
	if event == nil {
		return
	}
	guildID, ok := r.internalGuildID(event.GuildID)
	if ok {
		r.submitBulkDelete(bulkDeleteEvent{guildID: guildID, channelID: event.ChannelID, messageIDs: append([]string(nil), event.Messages...)})
	}
}

// onGuildMemberAdd queues configured member-join logging.
func (r *Runtime) onGuildMemberAdd(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
	r.memberEvent(event.Member, generallogging.MemberJoin)
}

// onGuildMemberRemove queues configured member-leave logging.
func (r *Runtime) onGuildMemberRemove(_ *discordgo.Session, event *discordgo.GuildMemberRemove) {
	r.memberEvent(event.Member, generallogging.MemberLeave)
}

// memberEvent maps one Discord member lifecycle event into module identity.
func (r *Runtime) memberEvent(member *discordgo.Member, eventType generallogging.EventType) {
	if member == nil {
		return
	}
	guildID, ok := r.internalGuildID(member.GuildID)
	if !ok {
		return
	}
	actorID := ""
	if member.User != nil {
		actorID = member.User.ID
	}
	r.submit(generallogging.Event{GuildID: guildID, Type: eventType, ActorDiscordUserID: actorID})
}

// onGuildBanAdd queues configured ban logging.
func (r *Runtime) onGuildBanAdd(_ *discordgo.Session, event *discordgo.GuildBanAdd) {
	r.banEvent(event.GuildID, event.User, generallogging.DiscordBan)
}

// onGuildBanRemove queues configured unban logging.
func (r *Runtime) onGuildBanRemove(_ *discordgo.Session, event *discordgo.GuildBanRemove) {
	r.banEvent(event.GuildID, event.User, generallogging.DiscordUnban)
}

// banEvent maps one Discord ban lifecycle event into module identity.
func (r *Runtime) banEvent(discordGuildID string, user *discordgo.User, eventType generallogging.EventType) {
	guildID, ok := r.internalGuildID(discordGuildID)
	if !ok {
		return
	}
	actorID := ""
	if user != nil {
		actorID = user.ID
	}
	r.submit(generallogging.Event{GuildID: guildID, Type: eventType, ActorDiscordUserID: actorID})
}

// onGuildUpdate queues non-content guild metadata changes.
func (r *Runtime) onGuildUpdate(_ *discordgo.Session, event *discordgo.GuildUpdate) {
	if event == nil || event.Guild == nil {
		return
	}
	guildID, ok := r.internalGuildID(event.ID)
	if ok {
		r.submit(generallogging.Event{GuildID: guildID, Type: generallogging.GuildChange, Metadata: map[string]string{"name": event.Name}})
	}
}

// onGuildDelete disables the departed guild's honeypot while retaining its
// configuration for an explicit repair after a future rejoin.
func (r *Runtime) onGuildDelete(_ *discordgo.Session, event *discordgo.GuildDelete) {
	if r == nil || r.registry == nil || r.HoneypotDiscord == nil || event == nil || event.Guild == nil || event.Unavailable {
		return
	}
	ctx := context.Background()
	guildID, err := r.resolver.internalIDAny(ctx, event.ID)
	if err != nil {
		return
	}
	configuration, err := r.registry.Configuration(ctx, guildID, modules.Honeypots)
	if err != nil || configuration == nil || !configuration.Enabled {
		return
	}
	var settings honeypot.Settings
	if json.Unmarshal([]byte(configuration.ConfigJSON), &settings) != nil || settings.ChannelDiscordID == "" {
		return
	}
	_ = r.HoneypotDiscord.HandleDeletedChannel(ctx, guildID, settings.ChannelDiscordID)
}

// onChannelCreate queues configured channel creation logging.
func (r *Runtime) onChannelCreate(_ *discordgo.Session, event *discordgo.ChannelCreate) {
	if event != nil {
		r.channelEvent(event.Channel, "created")
	}
}

// onChannelUpdate queues configured channel update logging.
func (r *Runtime) onChannelUpdate(_ *discordgo.Session, event *discordgo.ChannelUpdate) {
	if event != nil {
		r.channelEvent(event.Channel, "updated")
	}
}

// channelEvent maps one Discord channel lifecycle event into module identity.
func (r *Runtime) channelEvent(channel *discordgo.Channel, operation string) {
	if channel == nil || channel.GuildID == "" {
		return
	}
	guildID, ok := r.internalGuildID(channel.GuildID)
	if ok {
		r.submit(generallogging.Event{GuildID: guildID, ChannelDiscordID: channel.ID, Type: generallogging.ChannelChange, Metadata: map[string]string{"operation": operation, "name": channel.Name}})
	}
}

// onChannelDelete queues logging and repairs every module reference.
func (r *Runtime) onChannelDelete(_ *discordgo.Session, event *discordgo.ChannelDelete) {
	if event == nil || event.Channel == nil {
		return
	}
	r.channelEvent(event.Channel, "deleted")
	guildID, ok := r.internalGuildID(event.GuildID)
	if !ok {
		return
	}
	ctx := context.Background()
	if r.TicketDiscord != nil {
		_ = r.TicketDiscord.HandleDeletedEntryChannel(ctx, guildID, event.ID)
	}
	if r.HoneypotDiscord != nil {
		_ = r.HoneypotDiscord.HandleDeletedChannel(ctx, guildID, event.ID)
	}
	var ticket struct {
		ID      string
		GuildID string
	}
	result := r.db.WithContext(ctx).Table("tickets").Select("id, guild_id").Where("guild_id = ? AND thread_discord_channel_id = ?", guildID, event.ID).Limit(1).Find(&ticket)
	if r.TicketDiscord != nil && result.Error == nil && result.RowsAffected == 1 {
		_ = r.TicketDiscord.HandleDeletedChannel(ctx, ticket.GuildID, ticket.ID, event.ID)
	}
	if r.Logging != nil {
		_, _, _ = r.Logging.RepairDeletedChannel(ctx, generallogging.Actor{GuildID: guildID, DiscordUserID: "quack-system", CanManage: true}, event.ID)
	}
}

// cachedMessage copies the bounded subset allowed by logging privacy settings.
func cachedMessage(guildID string, message *discordgo.Message) generallogging.CachedMessage {
	cached := generallogging.CachedMessage{GuildID: guildID, ChannelDiscordID: message.ChannelID, MessageDiscordID: message.ID, Content: message.Content}
	for _, attachment := range message.Attachments {
		cached.Attachments = append(cached.Attachments, generallogging.AttachmentMetadata{Filename: attachment.Filename, ContentType: attachment.ContentType, Size: int64(attachment.Size)})
	}
	for _, embed := range message.Embeds {
		cached.EmbedTypes = append(cached.EmbedTypes, string(embed.Type))
	}
	return cached
}

// messageEvent copies one Discord message event into the ephemeral module shape.
func messageEvent(guildID string, eventType generallogging.EventType, message *discordgo.Message, before, after string) generallogging.Event {
	cached := cachedMessage(guildID, message)
	actorID := ""
	if message.Author != nil {
		actorID = message.Author.ID
	}
	return generallogging.Event{
		GuildID: guildID, ChannelDiscordID: message.ChannelID, MessageDiscordID: message.ID,
		ActorDiscordUserID: actorID, Type: eventType, Before: before, After: after,
		Attachments: cached.Attachments, EmbedTypes: cached.EmbedTypes,
	}
}
