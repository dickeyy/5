package actionmods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// DiscordClient defines the external operations needed by this package, keeping the concrete client at the adapter boundary.
type DiscordClient interface {
	SendDM(ctx context.Context, discordUserID, message string) (map[string]any, error)
}

// EnforcementClient is implemented by Discord adapters that can perform real moderation and reversal operations.
type EnforcementClient interface {
	TimeoutMember(context.Context, string, string, int, string) (map[string]any, error)
	KickMember(context.Context, string, string, string) (map[string]any, error)
	BanMember(context.Context, string, string, int, string) (map[string]any, error)
	RemoveMemberTimeout(context.Context, string, string, string) (map[string]any, error)
	UnbanMember(context.Context, string, string, string) (map[string]any, error)
}

// Context carries the request-scoped context data needed by downstream logic.
type Context struct {
	Case           model.Case
	Execution      model.CaseActionExecution
	Config         map[string]any
	DiscordGuildID string
}

// Result describes an action attempt in implementation-neutral terms so the action service can persist retries, failures, and external response data uniformly.
type Result struct {
	Retryable        bool
	ErrorCode        string
	Error            string
	Response         map[string]any
	OutcomeUncertain bool
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
	Code             string
	Message          string
	Retryable        bool
	OutcomeUncertain bool
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
			result := RetryableError(actionErr.Code, actionErr.Error())
			result.OutcomeUncertain = actionErr.OutcomeUncertain
			return result
		}
		result := PermanentError(actionErr.Code, actionErr.Error())
		result.OutcomeUncertain = actionErr.OutcomeUncertain
		return result
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Result{ErrorCode: "context_cancelled", Error: "Discord request was interrupted", OutcomeUncertain: true}
	}
	return Result{ErrorCode: "discord_error", Error: "Discord request failed", OutcomeUncertain: true}
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

// ConfigInt reads integer-valued JSON settings without accepting fractional values.
func ConfigInt(config map[string]any, key string) int {
	value, ok := config[key]
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case float64:
		bound := math.Ldexp(1, strconv.IntSize-1)
		if math.IsNaN(typed) || math.IsInf(typed, 0) || math.Trunc(typed) != typed || typed < -bound || typed >= bound {
			return 0
		}
		return int(typed)
	case int:
		return typed
	case json.Number:
		parsed, _ := strconv.Atoi(typed.String())
		return parsed
	default:
		parsed, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(typed)))
		return parsed
	}
}

// AuditReason returns a bounded Discord audit-log reason containing the immutable case reference and official reason.
func AuditReason(action Context) string {
	value := fmt.Sprintf("Quack case #%d: %s", action.Case.CaseNumber, strings.TrimSpace(action.Case.Reason))
	runes := []rune(value)
	if len(runes) > 512 {
		return string(runes[:512])
	}
	return value
}
