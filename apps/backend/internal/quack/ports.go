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
	GetGuildByID(context.Context, string) (*model.Guild, error)
	UpsertGuild(context.Context, model.UpsertGuildParams) (*model.Guild, error)
	BootstrapGuild(context.Context, model.BootstrapGuildParams) (*model.BootstrapGuildResult, error)
	DeactivateGuild(context.Context, string, *model.AuditLogEntry) (*model.Guild, error)
	GetGuildSettings(context.Context, string) (*model.GuildSettings, error)
	UpdateGuildSettings(context.Context, model.UpdateGuildSettingsParams) (*model.GuildSettings, error)
	ClearGuildChannelReferences(context.Context, string, string, *model.AuditLogEntry) (*model.GuildSettings, error)
	UpsertStaffMember(context.Context, model.UpsertStaffMemberParams) (*model.StaffMember, error)
	GetStaffMember(context.Context, string, string) (*model.StaffMember, error)
	CreateCaseTemplate(context.Context, model.CreateCaseTemplateParams) (*model.ExpandedCaseTemplate, error)
	ListCaseTemplates(context.Context, string) ([]model.ExpandedCaseTemplate, error)
	GetCaseTemplateExpanded(context.Context, string, string) (*model.ExpandedCaseTemplate, error)
	GetCaseTemplateBySlug(context.Context, string, string) (*model.CaseTemplate, error)
	UpdateCaseTemplate(context.Context, model.UpdateCaseTemplateParams) (*model.ExpandedCaseTemplate, error)
	ArchiveCaseTemplate(context.Context, string, string, *model.AuditLogEntry) (*model.ExpandedCaseTemplate, error)
	RestoreCaseTemplate(context.Context, string, string, *model.AuditLogEntry) (*model.ExpandedCaseTemplate, error)
	CreateAuditLogEntry(context.Context, *model.AuditLogEntry) error
	ListAuditLogEntriesFiltered(context.Context, model.ListAuditLogEntriesParams) (*model.ListAuditLogEntriesResult, error)
	CreateCase(context.Context, model.CreateCaseParams) (*model.CreatedCase, error)
	CountTemplateCasesForTarget(context.Context, model.CountTemplateCasesForTargetParams) (int64, error)
	ListCasesFiltered(context.Context, model.ListCasesParams) (*model.ListCasesResult, error)
	GetCaseByIDOrNumber(context.Context, string, string) (*model.Case, error)
	GetCaseByID(context.Context, string) (*model.Case, error)
	GetAppealByCaseID(context.Context, string) (*model.Appeal, error)
	GetCaseByIdempotencyKey(context.Context, string, string) (*model.Case, error)
	VoidCase(context.Context, model.VoidCaseParams) (*model.Case, error)
	TargetCaseSummary(context.Context, string, string) (*model.TargetCaseSummary, error)
	ListCaseEvents(context.Context, string) ([]model.CaseEvent, error)
	ListCaseActionExecutions(context.Context, string) ([]model.CaseActionExecution, error)
	ListCaseActionAttempts(context.Context, []string) ([]model.CaseActionAttempt, error)
	GetCaseActionExecution(context.Context, string, string) (*model.CaseActionExecution, error)
	ListCaseEvidence(context.Context, string) ([]model.CaseEvidenceSnapshot, []model.CaseEvidenceAttachment, error)
	GetCaseNotification(context.Context, string) (*model.CaseNotification, error)
	PrepareCaseNotification(context.Context, string, string, string) error
	ClaimNextCaseAction(context.Context, model.ClaimCaseActionParams) (*model.ClaimedCaseAction, error)
	CompleteCaseAction(context.Context, model.CompleteCaseActionParams) error
	ListFailedCaseActions(context.Context, model.FailedCaseActionFilter) (*model.FailedCaseActionResult, error)
	RetryCaseAction(context.Context, model.RetryCaseActionParams) (*model.CaseActionExecution, error)
	DismissCaseAction(context.Context, model.DismissCaseActionParams) (*model.CaseActionExecution, error)
	QueueCaseReversal(context.Context, model.QueueCaseReversalParams) (*model.CaseActionExecution, error)
	ClaimCaseNotification(context.Context, model.ClaimCaseNotificationParams) (*model.CaseNotification, error)
	BeginCaseNotificationDelivery(context.Context, string, string) error
	CompleteCaseNotification(context.Context, model.CompleteCaseNotificationParams) error
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
