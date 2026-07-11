package actionmods

import "context"

// BanUser returns the explicit unsupported executor for bans. Keeping the action visible as a permanent failure preserves case history without pretending Discord enforcement succeeded.
func BanUser() Executor {
	return Func(func(ctx context.Context, action Context) Result {
		_ = ctx
		_ = action
		return PermanentError("action_not_implemented", "ban_user action module is not implemented")
	})
}
