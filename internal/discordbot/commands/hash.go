package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/bwmarrin/discordgo"
)

// canonicalCommand groups the canonical command state used to keep this package's responsibilities explicit.
type canonicalCommand struct {
	Type                     discordgo.ApplicationCommandType        `json:"type"`
	Name                     string                                  `json:"name"`
	NameLocalizations        *map[discordgo.Locale]string            `json:"name_localizations,omitempty"`
	Description              string                                  `json:"description,omitempty"`
	DescriptionLocalizations *map[discordgo.Locale]string            `json:"description_localizations,omitempty"`
	DefaultPermission        *bool                                   `json:"default_permission,omitempty"`
	DefaultMemberPermissions *int64                                  `json:"default_member_permissions,omitempty"`
	DMPermission             *bool                                   `json:"dm_permission,omitempty"`
	NSFW                     *bool                                   `json:"nsfw,omitempty"`
	Contexts                 *[]discordgo.InteractionContextType     `json:"contexts,omitempty"`
	IntegrationTypes         *[]discordgo.ApplicationIntegrationType `json:"integration_types,omitempty"`
	Options                  []canonicalCommandOption                `json:"options,omitempty"`
}

// canonicalCommandOption groups the canonical command option state used to keep this package's responsibilities explicit.
type canonicalCommandOption struct {
	Type                     discordgo.ApplicationCommandOptionType `json:"type"`
	Name                     string                                 `json:"name"`
	NameLocalizations        map[discordgo.Locale]string            `json:"name_localizations,omitempty"`
	Description              string                                 `json:"description,omitempty"`
	DescriptionLocalizations map[discordgo.Locale]string            `json:"description_localizations,omitempty"`
	ChannelTypes             []discordgo.ChannelType                `json:"channel_types,omitempty"`
	Required                 bool                                   `json:"required,omitempty"`
	Options                  []canonicalCommandOption               `json:"options,omitempty"`
	Autocomplete             bool                                   `json:"autocomplete,omitempty"`
	Choices                  []canonicalCommandOptionChoice         `json:"choices,omitempty"`
	MinValue                 *float64                               `json:"min_value,omitempty"`
	MaxValue                 float64                                `json:"max_value,omitempty"`
	MinLength                *int                                   `json:"min_length,omitempty"`
	MaxLength                int                                    `json:"max_length,omitempty"`
}

// canonicalCommandOptionChoice groups the canonical command option choice state used to keep this package's responsibilities explicit.
type canonicalCommandOptionChoice struct {
	Name              string                      `json:"name"`
	NameLocalizations map[discordgo.Locale]string `json:"name_localizations,omitempty"`
	Value             any                         `json:"value"`
}

// CommandHash computes a stable digest for command hash so unchanged Discord commands can skip synchronization.
func CommandHash(command *discordgo.ApplicationCommand) (string, error) {
	hash, _, err := CommandFingerprint(command)
	return hash, err
}

// CommandFingerprint computes a stable digest for command fingerprint so unchanged Discord commands can skip synchronization.
func CommandFingerprint(command *discordgo.ApplicationCommand) (string, string, error) {
	canonical := canonicalizeCommand(command)
	body, err := json.Marshal(canonical)
	if err != nil {
		return "", "", err
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), string(body), nil
}

// canonicalizeCommand produces a stable command representation for deterministic validation, comparison, or caching.
func canonicalizeCommand(command *discordgo.ApplicationCommand) canonicalCommand {
	if command == nil {
		return canonicalCommand{}
	}

	commandType := command.Type
	if commandType == 0 {
		commandType = discordgo.ChatApplicationCommand
	}

	return canonicalCommand{
		Type:                     commandType,
		Name:                     command.Name,
		NameLocalizations:        command.NameLocalizations,
		Description:              command.Description,
		DescriptionLocalizations: command.DescriptionLocalizations,
		DefaultPermission:        command.DefaultPermission,
		DefaultMemberPermissions: command.DefaultMemberPermissions,
		DMPermission:             command.DMPermission,
		NSFW:                     normalizedBoolPointer(command.NSFW),
		Contexts:                 command.Contexts,
		IntegrationTypes:         normalizedIntegrationTypes(command.IntegrationTypes),
		Options:                  canonicalizeOptions(command.Options),
	}
}

// canonicalizeOptions produces a stable options representation for deterministic validation, comparison, or caching.
func canonicalizeOptions(options []*discordgo.ApplicationCommandOption) []canonicalCommandOption {
	if len(options) == 0 {
		return nil
	}

	out := make([]canonicalCommandOption, 0, len(options))
	for _, option := range options {
		if option == nil {
			continue
		}

		out = append(out, canonicalCommandOption{
			Type:                     option.Type,
			Name:                     option.Name,
			NameLocalizations:        option.NameLocalizations,
			Description:              option.Description,
			DescriptionLocalizations: option.DescriptionLocalizations,
			ChannelTypes:             option.ChannelTypes,
			Required:                 option.Required,
			Options:                  canonicalizeOptions(option.Options),
			Autocomplete:             option.Autocomplete,
			Choices:                  canonicalizeChoices(option.Choices),
			MinValue:                 option.MinValue,
			MaxValue:                 option.MaxValue,
			MinLength:                option.MinLength,
			MaxLength:                option.MaxLength,
		})
	}

	return out
}

// normalizedBoolPointer produces a stable normalized bool pointer representation for deterministic validation, comparison, or caching.
func normalizedBoolPointer(value *bool) *bool {
	if value == nil || !*value {
		return nil
	}
	return value
}

// normalizedIntegrationTypes produces a stable normalized integration types representation for deterministic validation, comparison, or caching.
func normalizedIntegrationTypes(value *[]discordgo.ApplicationIntegrationType) *[]discordgo.ApplicationIntegrationType {
	if value == nil || len(*value) == 0 {
		return nil
	}

	copied := append([]discordgo.ApplicationIntegrationType(nil), (*value)...)
	sort.Slice(copied, func(i, j int) bool {
		return copied[i] < copied[j]
	})
	if len(copied) == 2 &&
		copied[0] == discordgo.ApplicationIntegrationGuildInstall &&
		copied[1] == discordgo.ApplicationIntegrationUserInstall {
		return nil
	}
	return &copied
}

// canonicalizeChoices produces a stable choices representation for deterministic validation, comparison, or caching.
func canonicalizeChoices(choices []*discordgo.ApplicationCommandOptionChoice) []canonicalCommandOptionChoice {
	if len(choices) == 0 {
		return nil
	}

	out := make([]canonicalCommandOptionChoice, 0, len(choices))
	for _, choice := range choices {
		if choice == nil {
			continue
		}

		out = append(out, canonicalCommandOptionChoice{
			Name:              choice.Name,
			NameLocalizations: choice.NameLocalizations,
			Value:             choice.Value,
		})
	}

	return out
}
