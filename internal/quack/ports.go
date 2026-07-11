package quack

import (
	"context"
	"time"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// Repository defines the persistence operations the repository consumer requires.
type Repository interface {
	SaveOAuthState(context.Context, string, *model.OAuthState, time.Duration) error
	ConsumeOAuthState(context.Context, string) (*model.OAuthState, error)
	SaveSession(context.Context, *model.AuthSession, time.Duration) error
	GetSession(context.Context, string) (*model.AuthSession, error)
	DeleteSession(context.Context, string) error
	RefreshSessionTTL(context.Context, string, time.Duration) error
	GetGuildByDiscordID(context.Context, string) (*model.Guild, error)
	UpsertGuild(context.Context, model.UpsertGuildParams) (*model.Guild, error)
	UpsertStaffMember(context.Context, model.UpsertStaffMemberParams) (*model.StaffMember, error)
	GetStaffMember(context.Context, string, string) (*model.StaffMember, error)
	CreateCaseTemplate(context.Context, model.CreateCaseTemplateParams) (*model.ExpandedCaseTemplate, error)
	ListCaseTemplates(context.Context, string) ([]model.ExpandedCaseTemplate, error)
	GetCaseTemplateExpanded(context.Context, string, string) (*model.ExpandedCaseTemplate, error)
	GetCaseTemplateBySlug(context.Context, string, string) (*model.CaseTemplate, error)
	UpdateCaseTemplate(context.Context, model.UpdateCaseTemplateParams) (*model.ExpandedCaseTemplate, error)
	ArchiveCaseTemplate(context.Context, string, string, *model.AuditLogEntry) (*model.ExpandedCaseTemplate, error)
	CreateAuditLogEntry(context.Context, *model.AuditLogEntry) error
	ListAuditLogEntriesFiltered(context.Context, model.ListAuditLogEntriesParams) (*model.ListAuditLogEntriesResult, error)
	CreateCase(context.Context, model.CreateCaseParams) (*model.CreatedCase, error)
	CountTemplateCasesForTarget(context.Context, model.CountTemplateCasesForTargetParams) (int64, error)
	ListCasesFiltered(context.Context, model.ListCasesParams) (*model.ListCasesResult, error)
	GetCaseByIDOrNumber(context.Context, string, string) (*model.Case, error)
	TargetCaseSummary(context.Context, string, string) (*model.TargetCaseSummary, error)
	ListCaseEvents(context.Context, string) ([]model.CaseEvent, error)
	ListCaseActionExecutions(context.Context, string) ([]model.CaseActionExecution, error)
	ListCaseActionAttempts(context.Context, []string) ([]model.CaseActionAttempt, error)
	ClaimNextCaseAction(context.Context, model.ClaimCaseActionParams) (*model.ClaimedCaseAction, error)
	CompleteCaseAction(context.Context, model.CompleteCaseActionParams) error
	SkipCaseActions(context.Context, model.SkipCaseActionsParams) error
	ListExecutableCaseIDs(context.Context, int) ([]string, error)
	ActionQueueSnapshot(context.Context, string, int) (*model.ActionQueueSnapshot, error)
	WithGuildCaseLock(context.Context, string, func(Repository) error) error
	PingDatabase(context.Context) error
	PingRedis(context.Context) error
	HashGet(context.Context, string, string) ([]byte, error)
	HashSet(context.Context, string, string, []byte) error
}

// CaseWorkScheduler schedules case work while hiding the queue implementation from the application core.
type CaseWorkScheduler interface {
	Submit(context.Context, string) bool
	Stats() QueueStats
}

// QueueStats groups the queue stats state used to keep this package's responsibilities explicit.
type QueueStats struct {
	BufferSize        int    `json:"buffer_size"`
	Workers           int    `json:"workers"`
	Active            bool   `json:"active"`
	QueueSize         int    `json:"queue_size"`
	EnqueuedTotal     uint64 `json:"enqueued_total"`
	DroppedTotal      uint64 `json:"dropped_total"`
	ProcessedTotal    uint64 `json:"processed_total"`
	FailedTotal       uint64 `json:"failed_total"`
	PanickedTotal     uint64 `json:"panicked_total"`
	LastProcessedID   string `json:"last_processed_id,omitempty"`
	LastProcessedType string `json:"last_processed_type,omitempty"`
}
