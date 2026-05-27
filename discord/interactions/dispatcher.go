package interactions

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/discord/ui"
	"github.com/rs/zerolog/log"
)

type CommandLookup interface {
	LookupCommand(name string) (ui.Handler, bool)
}

type Client interface {
	InteractionRespond(*discordgo.Interaction, *discordgo.InteractionResponse) error
	InteractionResponseEdit(*discordgo.Interaction, *discordgo.WebhookEdit) (*discordgo.Message, error)
	FollowupMessageCreate(*discordgo.Interaction, bool, *discordgo.WebhookParams) (*discordgo.Message, error)
	InteractionResponseDelete(*discordgo.Interaction) error
}

type Dispatcher struct {
	Services   *app.Services
	Commands   CommandLookup
	Components *ComponentRegistry
	Client     Client
}

func NewDispatcher(services *app.Services, commands CommandLookup) *Dispatcher {
	return &Dispatcher{
		Services:   services,
		Commands:   commands,
		Components: NewComponentRegistry(),
	}
}

func (d *Dispatcher) Handle(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if interaction == nil || interaction.Interaction == nil {
		return
	}
	if d.Client == nil {
		d.Client = sessionClient{session: session}
	}

	switch interaction.Type {
	case discordgo.InteractionApplicationCommand, discordgo.InteractionApplicationCommandAutocomplete:
		d.handleCommand(session, interaction)
	case discordgo.InteractionMessageComponent:
		d.handleComponent(session, interaction)
	case discordgo.InteractionModalSubmit:
		d.handleModal(session, interaction)
	}
}

func (d *Dispatcher) handleCommand(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if d.Commands == nil {
		return
	}
	data := interaction.ApplicationCommandData()
	handler, ok := d.Commands.LookupCommand(data.Name)
	if !ok {
		return
	}
	d.execute(session, interaction, data.Name, handler)
}

func (d *Dispatcher) handleComponent(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if d.Components == nil {
		_ = d.respond(interaction, ui.Error("That component is not available."))
		return
	}
	data := interaction.MessageComponentData()
	handler, ok, err := d.Components.LookupComponent(data.CustomID)
	if err != nil || !ok {
		_ = d.respond(interaction, ui.Error("That component is not available."))
		return
	}
	d.execute(session, interaction, "component:"+data.CustomID, handler)
}

func (d *Dispatcher) handleModal(session *discordgo.Session, interaction *discordgo.InteractionCreate) {
	if d.Components == nil {
		_ = d.respond(interaction, ui.Error("That modal is not available."))
		return
	}
	data := interaction.ModalSubmitData()
	handler, ok, err := d.Components.LookupModal(data.CustomID)
	if err != nil || !ok {
		_ = d.respond(interaction, ui.Error("That modal is not available."))
		return
	}
	d.execute(session, interaction, "modal:"+data.CustomID, handler)
}

func (d *Dispatcher) execute(session *discordgo.Session, interaction *discordgo.InteractionCreate, name string, handler ui.Handler) {
	ctx := interactionTraceContext(interaction)
	result := d.safeHandle(ctx, session, interaction, name, handler)
	if result.Response == nil {
		return
	}
	if err := d.respond(interaction, result.Response); err != nil {
		log.Error().Err(err).Str("interaction", name).Msg("failed to respond to Discord interaction")
		return
	}
	if result.Task == nil {
		return
	}

	go d.runTask(ctx, interaction, name, result.Task)
}

func (d *Dispatcher) safeHandle(ctx context.Context, session *discordgo.Session, interaction *discordgo.InteractionCreate, name string, handler ui.Handler) (result ui.HandlerResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error().
				Str("interaction", name).
				Str("request_id", app.RequestIDFromContext(ctx)).
				Str("correlation_id", app.CorrelationIDFromContext(ctx)).
				Interface("panic", recovered).
				Bytes("stack", debug.Stack()).
				Msg("Discord interaction handler panicked")
			result = ui.Immediate(ui.Error("Quack could not handle that interaction."))
		}
	}()
	return handler(ui.Context{
		Context:     ctx,
		Services:    d.Services,
		Session:     session,
		Interaction: interaction,
	})
}

func (d *Dispatcher) runTask(ctx context.Context, interaction *discordgo.InteractionCreate, name string, task ui.Task) {
	defer func() {
		if recovered := recover(); recovered != nil {
			log.Error().
				Str("interaction", name).
				Str("request_id", app.RequestIDFromContext(ctx)).
				Str("correlation_id", app.CorrelationIDFromContext(ctx)).
				Interface("panic", recovered).
				Bytes("stack", debug.Stack()).
				Msg("Discord interaction task panicked")
			_, _ = d.responder(interaction).EditOriginal(ui.ErrorEdit("Quack could not finish that interaction."))
		}
	}()
	if err := task(ctx, d.responder(interaction)); err != nil {
		log.Error().
			Err(err).
			Str("interaction", name).
			Str("request_id", app.RequestIDFromContext(ctx)).
			Str("correlation_id", app.CorrelationIDFromContext(ctx)).
			Msg("Discord interaction task failed")
		_, _ = d.responder(interaction).EditOriginal(ui.ErrorEdit("Quack could not finish that interaction."))
	}
}

func interactionTraceContext(interaction *discordgo.InteractionCreate) context.Context {
	requestID := app.NewTraceID()
	correlationID := requestID
	if interaction != nil && interaction.ID != "" {
		requestID = "discord:" + interaction.ID
		correlationID = requestID
	}
	return app.ContextWithTrace(context.Background(), requestID, correlationID)
}

func (d *Dispatcher) respond(interaction *discordgo.InteractionCreate, response *discordgo.InteractionResponse) error {
	if d.Client == nil {
		return fmt.Errorf("discord interaction client is not configured")
	}
	return d.Client.InteractionRespond(interaction.Interaction, response)
}

func (d *Dispatcher) responder(interaction *discordgo.InteractionCreate) ui.Responder {
	return responder{client: d.Client, interaction: interaction.Interaction}
}

type responder struct {
	client      Client
	interaction *discordgo.Interaction
}

func (r responder) EditOriginal(edit ui.Edit) (*discordgo.Message, error) {
	return r.client.InteractionResponseEdit(r.interaction, edit.WebhookEdit())
}

func (r responder) Followup(message ui.Message) (*discordgo.Message, error) {
	return r.client.FollowupMessageCreate(r.interaction, true, message.WebhookParams())
}

func (r responder) DeleteOriginal() error {
	return r.client.InteractionResponseDelete(r.interaction)
}

func (r responder) UpdateMessage(edit ui.Edit) (*discordgo.Message, error) {
	return r.EditOriginal(edit)
}

type sessionClient struct {
	session *discordgo.Session
}

func (c sessionClient) InteractionRespond(interaction *discordgo.Interaction, response *discordgo.InteractionResponse) error {
	return c.session.InteractionRespond(interaction, response)
}

func (c sessionClient) InteractionResponseEdit(interaction *discordgo.Interaction, edit *discordgo.WebhookEdit) (*discordgo.Message, error) {
	return c.session.InteractionResponseEdit(interaction, edit)
}

func (c sessionClient) FollowupMessageCreate(interaction *discordgo.Interaction, wait bool, params *discordgo.WebhookParams) (*discordgo.Message, error) {
	return c.session.FollowupMessageCreate(interaction, wait, params)
}

func (c sessionClient) InteractionResponseDelete(interaction *discordgo.Interaction) error {
	return c.session.InteractionResponseDelete(interaction)
}

func Key(namespace, action string) string {
	return strings.TrimSpace(namespace) + ":" + strings.TrimSpace(action)
}
