package app

import (
	"context"
	"errors"
	"time"

	"github.com/quackdiscord/bot/services"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

type OpsService struct {
	store *storage.Store
}

type OpsStatusResponse struct {
	GeneratedAt time.Time                `json:"generated_at"`
	Scope       string                   `json:"scope"`
	GuildID     string                   `json:"guild_id,omitempty"`
	Queue       services.EventQueueStats `json:"queue"`
	Actions     OpsActionStatus          `json:"actions"`
}

type OpsActionStatus struct {
	Capabilities         []OpsActionCapability     `json:"capabilities"`
	StatusCounts         map[string]int64          `json:"status_counts"`
	OldestPendingOrRetry *OpsOldestActionExecution `json:"oldest_pending_or_retry,omitempty"`
	RecentFailures       []OpsRecentActionFailure  `json:"recent_failures"`
}

type OpsActionCapability struct {
	ActionType structs.ActionType `json:"action_type"`
	Executable bool               `json:"executable"`
	Status     string             `json:"status"`
}

type OpsOldestActionExecution struct {
	ID          string                        `json:"id"`
	CaseID      string                        `json:"case_id"`
	CaseNumber  uint64                        `json:"case_number"`
	ActionType  structs.ActionType            `json:"action_type"`
	Status      structs.ActionExecutionStatus `json:"status"`
	CreatedAt   time.Time                     `json:"created_at"`
	NextRetryAt *time.Time                    `json:"next_retry_at,omitempty"`
}

type OpsRecentActionFailure struct {
	ID            string                        `json:"id"`
	CaseID        string                        `json:"case_id"`
	CaseNumber    uint64                        `json:"case_number"`
	ActionType    structs.ActionType            `json:"action_type"`
	Status        structs.ActionExecutionStatus `json:"status"`
	LastErrorCode string                        `json:"last_error_code,omitempty"`
	LastError     string                        `json:"last_error,omitempty"`
	UpdatedAt     time.Time                     `json:"updated_at"`
}

func NewOpsService(store *storage.Store) *OpsService {
	return &OpsService{store: store}
}

func (s *OpsService) GlobalStatus(ctx context.Context) (*OpsStatusResponse, error) {
	return s.status(ctx, "", "global")
}

func (s *OpsService) GuildStatus(ctx context.Context, guildID string) (*OpsStatusResponse, error) {
	if guildID == "" {
		return nil, errors.New("guild id is required")
	}
	return s.status(ctx, guildID, "guild")
}

func (s *OpsService) status(ctx context.Context, guildID, scope string) (*OpsStatusResponse, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("ops service is not configured")
	}
	snapshot, err := s.store.ActionQueueSnapshot(ctx, guildID, 10)
	if err != nil {
		return nil, err
	}

	queueStats := services.EventQueueStats{}
	if services.EQ != nil {
		queueStats = services.EQ.Stats()
	}

	return &OpsStatusResponse{
		GeneratedAt: time.Now().UTC(),
		Scope:       scope,
		GuildID:     guildID,
		Queue:       queueStats,
		Actions:     opsActionStatus(snapshot),
	}, nil
}

func opsActionStatus(snapshot *storage.ActionQueueSnapshot) OpsActionStatus {
	status := OpsActionStatus{
		Capabilities: actionCapabilities(),
		StatusCounts: map[string]int64{},
	}
	if snapshot == nil {
		return status
	}
	for _, row := range snapshot.StatusCounts {
		status.StatusCounts[string(row.Status)] = row.Count
	}
	if snapshot.OldestPendingOrRetry != nil {
		oldest := snapshot.OldestPendingOrRetry
		status.OldestPendingOrRetry = &OpsOldestActionExecution{
			ID:          oldest.ID,
			CaseID:      oldest.CaseID,
			CaseNumber:  oldest.CaseNumber,
			ActionType:  oldest.ActionType,
			Status:      oldest.Status,
			CreatedAt:   oldest.CreatedAt,
			NextRetryAt: oldest.NextRetryAt,
		}
	}
	status.RecentFailures = make([]OpsRecentActionFailure, 0, len(snapshot.RecentFailures))
	for _, failure := range snapshot.RecentFailures {
		status.RecentFailures = append(status.RecentFailures, OpsRecentActionFailure{
			ID:            failure.ID,
			CaseID:        failure.CaseID,
			CaseNumber:    failure.CaseNumber,
			ActionType:    failure.ActionType,
			Status:        failure.Status,
			LastErrorCode: failure.LastErrorCode,
			LastError:     failure.LastError,
			UpdatedAt:     failure.UpdatedAt,
		})
	}
	return status
}

func actionCapabilities() []OpsActionCapability {
	return []OpsActionCapability{
		{ActionType: structs.ActionSendDM, Executable: true, Status: "implemented"},
		{ActionType: structs.ActionTimeoutUser, Executable: false, Status: "not_implemented"},
		{ActionType: structs.ActionKickUser, Executable: false, Status: "not_implemented"},
		{ActionType: structs.ActionBanUser, Executable: false, Status: "not_implemented"},
	}
}
