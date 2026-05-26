package actions

import "context"

func KickUser() Executor {
	return Func(func(ctx context.Context, action Context) Result {
		_ = ctx
		_ = action
		return PermanentError("action_not_implemented", "kick_user action module is not implemented")
	})
}
