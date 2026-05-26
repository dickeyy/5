package actions

import "context"

func TimeoutUser() Executor {
	return Func(func(ctx context.Context, action Context) Result {
		_ = ctx
		_ = action
		return PermanentError("action_not_implemented", "timeout_user action module is not implemented")
	})
}
