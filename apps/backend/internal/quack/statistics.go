package quack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
)

var (
	ErrStatisticsValidation       = errors.New("statistics validation failed")
	ErrStatisticsPermissionDenied = errors.New("statistics permission denied")
)

// StatisticsInput defines an inclusive start and exclusive end for derived staff statistics.
type StatisticsInput struct {
	From string
	To   string
}

// StatisticsRepository derives operational statistics from immutable history.
type StatisticsRepository interface {
	CreateAuditLogEntry(context.Context, *model.AuditLogEntry) error
	DeriveStaffStatistics(context.Context, model.StaffStatisticsParams) (*model.StaffStatistics, error)
}

// StaffStatisticsService derives guild-scoped operational counts without persisting aggregates or rankings.
type StaffStatisticsService struct {
	store StatisticsRepository
}

// NewStaffStatisticsService constructs the derived statistics capability over the existing source-of-truth repository.
func NewStaffStatisticsService(store StatisticsRepository) *StaffStatisticsService {
	return &StaffStatisticsService{store: store}
}

// Get returns a guild-scoped derived snapshot for an authorized moderator.
func (s *StaffStatisticsService) Get(ctx context.Context, guildContext *GuildStaffContext, input StatisticsInput) (*model.StaffStatistics, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("statistics service is not configured")
	}
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil || !guildContext.Can(model.PermissionActionAuditRead) {
		s.auditRead(ctx, guildContext, model.AuditResultDenied, "permission_denied")
		return nil, ErrStatisticsPermissionDenied
	}
	from, to, err := statisticsRange(input, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	result, err := s.store.DeriveStaffStatistics(ctx, model.StaffStatisticsParams{GuildID: guildContext.Guild.ID, From: from, To: to})
	if err != nil {
		s.auditRead(ctx, guildContext, model.AuditResultFailure, "query_failed")
		return nil, err
	}
	if err := s.auditRead(ctx, guildContext, model.AuditResultSuccess, ""); err != nil {
		return nil, err
	}
	return result, nil
}

func statisticsRange(input StatisticsInput, now time.Time) (time.Time, time.Time, error) {
	to := now.UTC()
	var from time.Time
	var err error
	if strings.TrimSpace(input.To) != "" {
		to, err = time.Parse(time.RFC3339, strings.TrimSpace(input.To))
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: to must use RFC3339", ErrStatisticsValidation)
		}
	}
	from = to.AddDate(0, -1, 0)
	if strings.TrimSpace(input.From) != "" {
		from, err = time.Parse(time.RFC3339, strings.TrimSpace(input.From))
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("%w: from must use RFC3339", ErrStatisticsValidation)
		}
	}
	from, to = from.UTC(), to.UTC()
	if !from.Before(to) || to.Sub(from) > 366*24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: range must be positive and at most 366 days", ErrStatisticsValidation)
	}
	return from, to, nil
}

func (s *StaffStatisticsService) auditRead(ctx context.Context, guildContext *GuildStaffContext, result model.AuditResult, failure string) error {
	if guildContext == nil || guildContext.Guild == nil {
		return nil
	}
	actorID := ""
	bits := uint64(0)
	if guildContext.Staff != nil {
		actorID = guildContext.Staff.DiscordUserID
		bits = guildContext.PermissionBits
	}
	requestID, correlationID := TraceIDsFromContext(ctx)
	return recordAudit(ctx, s.store, &model.AuditLogEntry{GuildID: guildContext.Guild.ID, ActorDiscordUserID: actorID, ActorPermissionBits: bits, Source: model.AuditSourceAPI, Action: string(model.AuditActionStatisticsRead), ResourceType: "statistics", ResourceID: "guild", Result: result, FailureReason: failure, RequestID: requestID, CorrelationID: correlationID, MetadataJSON: "{}"})
}
