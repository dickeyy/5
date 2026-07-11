package actionmods

import (
	"context"
	"fmt"
)

// SendDM returns the operational direct-message executor. It uses a template-provided message when present and otherwise sends a case-reason fallback through the Discord port.
func SendDM(client DiscordClient) Executor {
	return Func(func(ctx context.Context, action Context) Result {
		if client == nil {
			return PermanentError("discord_unavailable", "discord action client is not configured")
		}

		message := ConfigString(action.Config, "message")
		if message == "" {
			message = fmt.Sprintf("You received a moderation case in this server: %s", action.Case.Reason)
		}

		response, err := client.SendDM(ctx, action.Case.TargetDiscordUserID, message)
		if err != nil {
			return ResultFromError(err)
		}
		return Result{Response: response}
	})
}
