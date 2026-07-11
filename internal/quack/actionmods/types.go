package actionmods

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// DiscordClient defines the external operations needed by this package, keeping the concrete client at the adapter boundary.
type DiscordClient interface {
	SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error)
}

// Context carries the request-scoped context data needed by downstream logic.
type Context struct {
	Case      model.Case
	Execution model.CaseActionExecution
	Config    map[string]any
}

// Result describes an action attempt in implementation-neutral terms so the action service can persist retries, failures, and external response data uniformly.
type Result struct {
	Retryable bool
	ErrorCode string
	Error     string
	Response  map[string]any
}

// Executor runs one action module without exposing Discord or persistence details to the orchestration service.
type Executor interface {
	Execute(ctx context.Context, action Context) Result
}

// Func adapts a function to Executor, keeping small action modules declarative.
type Func func(ctx context.Context, action Context) Result

// Execute invokes the wrapped action function; retry policy remains the responsibility of the caller.
func (f Func) Execute(ctx context.Context, action Context) Result {
	return f(ctx, action)
}

// DiscordError carries classified discord error failure details across package boundaries.
type DiscordError struct {
	Code      string
	Message   string
	Retryable bool
}

// Error formats discord error as a standard Go error without discarding its classification.
func (e DiscordError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

// ResultFromError encapsulates the result from error rule so callers share one consistent package implementation.
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

// PermanentError encapsulates the permanent error rule so callers share one consistent package implementation.
func PermanentError(code, message string) Result {
	return Result{ErrorCode: code, Error: message}
}

// RetryableError encapsulates the retryable error rule so callers share one consistent package implementation.
func RetryableError(code, message string) Result {
	return Result{Retryable: true, ErrorCode: code, Error: message}
}

// Unsupported encapsulates the unsupported rule so callers share one consistent package implementation.
func Unsupported(ctx context.Context, action Context) Result {
	_ = ctx
	return PermanentError("unsupported_action", fmt.Sprintf("action type %s is not supported", action.Execution.ActionType))
}

// ConfigString encapsulates the config string rule so callers share one consistent package implementation.
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
