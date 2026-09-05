package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/quackdiscord/bot/internal/quack/model"
	"gorm.io/gorm"
)

// ActionStatusCount aliases the core action status count contract so Store satisfies the port without maintaining a second data shape.
type ActionStatusCount = model.ActionStatusCount

// OldestActionExecution aliases the core oldest action execution contract so Store satisfies the port without maintaining a second data shape.
type OldestActionExecution = model.OldestActionExecution

// RecentActionFailure aliases the core recent action failure contract so Store satisfies the port without maintaining a second data shape.
type RecentActionFailure = model.RecentActionFailure

// ActionQueueSnapshot aliases the core action queue snapshot contract so Store satisfies the port without maintaining a second data shape.
type ActionQueueSnapshot = model.ActionQueueSnapshot

// ActionQueueSnapshot encapsulates the action queue snapshot rule so callers share one consistent package implementation.
func (s *Store) ActionQueueSnapshot(ctx context.Context, guildID string, recentLimit int) (*ActionQueueSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, errors.New("database not connected")
	}
	if recentLimit <= 0 {
		recentLimit = 10
	}
	if recentLimit > 25 {
		recentLimit = 25
	}

	base := s.db.WithContext(ctx).
		Table("case_action_executions AS e").
		Joins("JOIN cases AS c ON c.id = e.case_id")
	if guildID != "" {
		base = base.Where("c.guild_id = ?", guildID)
	}

	var statusCounts []ActionStatusCount
	if err := base.Session(&gorm.Session{}).Select("e.status AS status, COUNT(*) AS count").Group("e.status").Scan(&statusCounts).Error; err != nil {
		return nil, fmt.Errorf("count action executions by status: %w", err)
	}

	var oldest OldestActionExecution
	oldestResult := base.Session(&gorm.Session{}).
		Select("e.id, e.case_id, c.case_number, e.action_type, e.status, e.created_at, e.next_retry_at").
		Where("e.status IN ?", []model.ActionExecutionStatus{model.ActionExecutionPending, model.ActionExecutionRetrying}).
		Order("COALESCE(e.next_retry_at, e.created_at) ASC").
		Limit(1).
		Scan(&oldest)
	if oldestResult.Error != nil {
		return nil, fmt.Errorf("get oldest pending action: %w", oldestResult.Error)
	}

	var failures []RecentActionFailure
	if err := base.Session(&gorm.Session{}).
		Select("e.id, e.case_id, c.case_number, e.action_type, e.status, e.last_error_code, e.last_error, e.updated_at").
		Where("e.status = ? OR e.last_error_code <> ''", model.ActionExecutionFailed).
		Order("e.updated_at DESC").
		Limit(recentLimit).
		Scan(&failures).Error; err != nil {
		return nil, fmt.Errorf("list recent action failures: %w", err)
	}

	snapshot := &ActionQueueSnapshot{
		StatusCounts:   statusCounts,
		RecentFailures: failures,
	}
	if oldest.ID != "" {
		snapshot.OldestPendingOrRetry = &oldest
	}
	return snapshot, nil
}
