package model

// Label gives a human-readable outcome name for Discord and member messages.
// Persisted values and JSON contracts retain their stable action identifiers.
func (action ActionType) Label() string {
	switch action {
	case ActionTimeoutUser:
		return "Timeout"
	case ActionKickUser:
		return "Kick"
	case ActionBanUser:
		return "Ban"
	case ActionRemoveTimeout:
		return "Remove timeout"
	case ActionUnbanUser:
		return "Unban"
	case ActionSendDM:
		return "Notification"
	default:
		return "Action"
	}
}

// Label describes execution progress without exposing internal state names.
func (status ActionExecutionStatus) Label() string {
	switch status {
	case ActionExecutionPending:
		return "Queued"
	case ActionExecutionRunning:
		return "In progress"
	case ActionExecutionSucceeded:
		return "Completed"
	case ActionExecutionFailed:
		return "Needs review"
	case ActionExecutionRetrying:
		return "Retry scheduled"
	case ActionExecutionSkipped:
		return "Skipped"
	case ActionExecutionCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}
