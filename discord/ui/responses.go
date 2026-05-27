package ui

import (
	"context"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/app"
)

type Context struct {
	context.Context
	Services    *app.Services
	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
}

type Responder interface {
	EditOriginal(Edit) (*discordgo.Message, error)
	Followup(Message) (*discordgo.Message, error)
	DeleteOriginal() error
	UpdateMessage(Edit) (*discordgo.Message, error)
}

type Task func(context.Context, Responder) error

type HandlerResult struct {
	Response *discordgo.InteractionResponse
	Task     Task
}

type Handler func(Context) HandlerResult

func Immediate(response *discordgo.InteractionResponse) HandlerResult {
	return HandlerResult{Response: response}
}

func Async(response *discordgo.InteractionResponse, task Task) HandlerResult {
	return HandlerResult{Response: response, Task: task}
}

func Public(message Message) *discordgo.InteractionResponse {
	message.Ephemeral = false
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: message.ResponseData(),
	}
}

func Ephemeral(message Message) *discordgo.InteractionResponse {
	message.Ephemeral = true
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: message.ResponseData(),
	}
}

func DeferPublic() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
}

func DeferEphemeral() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
}

func DeferUpdate() *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}
}

func Update(message Message) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: message.ResponseData(),
	}
}

func Autocomplete(choices []*discordgo.ApplicationCommandOptionChoice) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	}
}

func Modal(title, customID string, components []discordgo.MessageComponent) *discordgo.InteractionResponse {
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			Title:      title,
			CustomID:   customID,
			Components: components,
		},
	}
}

func Error(content string) *discordgo.InteractionResponse {
	return Ephemeral(EmbedMessage(ErrorEmbed(content), true))
}

func ErrorEdit(content string) Edit {
	return EditMessage(EmbedMessage(ErrorEmbed(content), false))
}
