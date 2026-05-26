package actions

import (
	"context"
	"fmt"
)

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
