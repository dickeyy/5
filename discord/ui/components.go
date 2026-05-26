package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	ErrCustomIDInvalid = errors.New("custom id is invalid")
	ErrCustomIDTooLong = errors.New("custom id exceeds Discord limit")
)

type CustomID struct {
	Namespace string
	Action    string
	Version   string
	Payload   string
}

func EncodeCustomID(id CustomID) (string, error) {
	namespace := strings.TrimSpace(id.Namespace)
	action := strings.TrimSpace(id.Action)
	version := strings.TrimSpace(id.Version)
	if namespace == "" || action == "" || version == "" {
		return "", ErrCustomIDInvalid
	}
	if strings.Contains(namespace, ":") || strings.Contains(action, ":") || strings.Contains(version, ":") {
		return "", ErrCustomIDInvalid
	}

	value := namespace + ":" + action + ":" + version + ":" + strings.TrimSpace(id.Payload)
	if len([]rune(value)) > CustomIDLimit {
		return "", ErrCustomIDTooLong
	}
	return value, nil
}

func DecodeCustomID(value string) (CustomID, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 4)
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return CustomID{}, ErrCustomIDInvalid
	}
	if len([]rune(value)) > CustomIDLimit {
		return CustomID{}, ErrCustomIDTooLong
	}
	return CustomID{
		Namespace: parts[0],
		Action:    parts[1],
		Version:   parts[2],
		Payload:   parts[3],
	}, nil
}

func MustCustomID(id CustomID) string {
	value, err := EncodeCustomID(id)
	if err != nil {
		panic(fmt.Sprintf("invalid custom id: %v", err))
	}
	return value
}

func Button(customID, label string, style discordgo.ButtonStyle, disabled bool) discordgo.Button {
	return discordgo.Button{
		CustomID: customID,
		Label:    TruncateRunes(label, 80),
		Style:    style,
		Disabled: disabled,
	}
}

func LinkButton(url, label string, disabled bool) discordgo.Button {
	return discordgo.Button{
		URL:      url,
		Label:    TruncateRunes(label, 80),
		Style:    discordgo.LinkButton,
		Disabled: disabled,
	}
}

func Row(components ...discordgo.MessageComponent) discordgo.ActionsRow {
	if len(components) > 5 {
		components = components[:5]
	}
	return discordgo.ActionsRow{Components: components}
}

func Pagination(namespace, prefix, payload string, page, totalPages int) ([]discordgo.MessageComponent, error) {
	prevID, err := EncodeCustomID(CustomID{
		Namespace: namespace,
		Action:    prefix + "_prev",
		Version:   "v1",
		Payload:   payload,
	})
	if err != nil {
		return nil, err
	}
	nextID, err := EncodeCustomID(CustomID{
		Namespace: namespace,
		Action:    prefix + "_next",
		Version:   "v1",
		Payload:   payload,
	})
	if err != nil {
		return nil, err
	}

	return []discordgo.MessageComponent{
		Row(
			Button(prevID, "Prev", discordgo.SecondaryButton, page <= 1),
			Button(nextID, "Next", discordgo.PrimaryButton, page >= totalPages),
		),
	}, nil
}
