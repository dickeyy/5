package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/quackdiscord/bot/structs"
)

type DiscordClient interface {
	SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error)
}

type Context struct {
	Case      structs.Case
	Execution structs.CaseActionExecution
	Config    map[string]any
}

type Result struct {
	Retryable bool
	ErrorCode string
	Error     string
	Response  map[string]any
}

type Executor interface {
	Execute(ctx context.Context, action Context) Result
}

type Func func(ctx context.Context, action Context) Result

func (f Func) Execute(ctx context.Context, action Context) Result {
	return f(ctx, action)
}

type DiscordError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e DiscordError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

func ResultFromError(err error) Result {
	if err == nil {
		return Result{}
	}
	var actionErr DiscordError
	if errors.As(err, &actionErr) {
		if actionErr.Retryable {
			return RetryableError(actionErr.Code, actionErr.Error())
		}
		return PermanentError(actionErr.Code, actionErr.Error())
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return RetryableError("context_cancelled", err.Error())
	}
	return RetryableError("discord_error", err.Error())
}

func PermanentError(code, message string) Result {
	return Result{ErrorCode: code, Error: message}
}

func RetryableError(code, message string) Result {
	return Result{Retryable: true, ErrorCode: code, Error: message}
}

func Unsupported(ctx context.Context, action Context) Result {
	_ = ctx
	return PermanentError("unsupported_action", fmt.Sprintf("action type %s is not supported", action.Execution.ActionType))
}

func ConfigString(config map[string]any, key string) string {
	value, ok := config[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
