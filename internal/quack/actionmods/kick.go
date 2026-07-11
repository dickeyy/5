package actionmods

import "context"

// KickUser returns the explicit unsupported executor for kicks. Persisting that failure makes the configured action observable while the implementation remains intentionally unavailable.
func KickUser() Executor {
	return Func(func(ctx context.Context, action Context) Result {
		_ = ctx
		_ = action
		return PermanentError("action_not_implemented", "kick_user action module is not implemented")
	})
}
