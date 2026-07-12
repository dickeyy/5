package actionmods

import "context"

// KickUser executes the immutable case target and official reason through the Discord adapter.
func KickUser(client DiscordClient) Executor {
	return Func(func(ctx context.Context, action Context) Result {
		enforcement, ok := client.(EnforcementClient)
		if !ok {
			return PermanentError("discord_unavailable", "Discord enforcement is not configured")
		}
		response, err := enforcement.KickMember(ctx, action.DiscordGuildID, action.Case.TargetDiscordUserID, AuditReason(action))
		if err != nil {
			return ResultFromError(err)
		}
		return Result{Response: response}
	})
}
