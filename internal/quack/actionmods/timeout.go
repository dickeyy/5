package actionmods

import "context"

// TimeoutUser executes the exact template-owned duration through the Discord adapter.
func TimeoutUser(client DiscordClient) Executor {
	return Func(func(ctx context.Context, action Context) Result {
		enforcement, ok := client.(EnforcementClient)
		if !ok {
			return PermanentError("discord_unavailable", "Discord enforcement is not configured")
		}
		duration := ConfigInt(action.Config, "duration_seconds")
		if duration <= 0 {
			return PermanentError("invalid_action_config", "timeout duration is missing")
		}
		response, err := enforcement.TimeoutMember(ctx, action.DiscordGuildID, action.Case.TargetDiscordUserID, duration, AuditReason(action))
		if err != nil {
			return ResultFromError(err)
		}
		return Result{Response: response}
	})
}
