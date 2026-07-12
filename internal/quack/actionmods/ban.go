package actionmods

import "context"

// BanUser executes the exact Discord-supported history deletion window through the adapter.
func BanUser(client DiscordClient) Executor {
	return Func(func(ctx context.Context, action Context) Result {
		enforcement, ok := client.(EnforcementClient)
		if !ok {
			return PermanentError("discord_unavailable", "Discord enforcement is not configured")
		}
		response, err := enforcement.BanMember(ctx, action.DiscordGuildID, action.Case.TargetDiscordUserID, ConfigInt(action.Config, "delete_message_seconds"), AuditReason(action))
		if err != nil {
			return ResultFromError(err)
		}
		return Result{Response: response}
	})
}
