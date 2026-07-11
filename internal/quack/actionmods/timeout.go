package actionmods

import "context"

// TimeoutUser returns the explicit unsupported executor for timeouts so configured actions fail visibly instead of being silently ignored.
func TimeoutUser() Executor {
	return Func(func(ctx context.Context, action Context) Result {
		_ = ctx
		_ = action
		return PermanentError("action_not_implemented", "timeout_user action module is not implemented")
	})
}
