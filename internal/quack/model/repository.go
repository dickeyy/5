package model

import (
	"errors"
	"fmt"
	"time"
)

// ErrTemplateCompatibilityReviewRequired marks a preserved legacy template that cannot be represented safely by the live v5 policy contract.
var ErrTemplateCompatibilityReviewRequired = errors.New("template compatibility review required")

// TemplateCompatibilityReviewError identifies a quarantined template and explains why its preserved policy cannot be returned as live v5 configuration.
type TemplateCompatibilityReviewError struct {
	TemplateID string
	Reason     string
}

// Error returns an administrator-facing compatibility review message without projecting invalid legacy policy.
func (e *TemplateCompatibilityReviewError) Error() string {
	if e == nil {
		return ErrTemplateCompatibilityReviewRequired.Error()
	}
	if e.Reason == "" {
		return fmt.Sprintf("%s for template %s", ErrTemplateCompatibilityReviewRequired, e.TemplateID)
	}
	return fmt.Sprintf("%s for template %s: %s", ErrTemplateCompatibilityReviewRequired, e.TemplateID, e.Reason)
}

// Unwrap preserves sentinel matching across storage, service, and transport boundaries.
func (e *TemplateCompatibilityReviewError) Unwrap() error {
	return ErrTemplateCompatibilityReviewRequired
}

// ExpandedCaseTemplate groups the expanded case template state used to keep this package's responsibilities explicit.
type ExpandedCaseTemplate struct {
	Template CaseTemplate
	Levels   []ExpandedCaseTemplateLevel
}

// ExpandedCaseTemplateLevel groups the expanded case template level state used to keep this package's responsibilities explicit.
type ExpandedCaseTemplateLevel struct {
	Level   CaseTemplateLevel
	Actions []CaseTemplateLevelAction
}

// CreateCaseTemplateParams groups the validated inputs needed for create case template params.
type CreateCaseTemplateParams struct {
	Template CaseTemplate
	Levels   []ExpandedCaseTemplateLevel
	Audit    *AuditLogEntry
}

// UpdateCaseTemplateParams groups the validated inputs needed for update case template params.
type UpdateCaseTemplateParams struct {
	GuildID, TemplateID string
	Template            CaseTemplate
	Levels              []ExpandedCaseTemplateLevel
	Audit               *AuditLogEntry
}

// ListAuditLogEntriesParams groups the validated inputs needed for list audit log entries params.
type ListAuditLogEntriesParams struct {
	GuildID, ActorDiscordUserID, Action, ResourceType, ResourceID string
	Result                                                        AuditResult
	Limit, Offset                                                 int
}

// ListAuditLogEntriesResult captures the outcome of list audit log entries result for the caller.
type ListAuditLogEntriesResult struct {
	Entries []AuditLogEntry
	Total   int64
}

// CreateCaseParams groups the validated inputs needed for create case params.
type CreateCaseParams struct {
	Case             Case
	Event            CaseEvent
	ActionExecutions []CaseActionExecution
	Audit            *AuditLogEntry
}

// CreatedCase groups the created case state used to keep this package's responsibilities explicit.
type CreatedCase struct {
	Case             Case
	Event            CaseEvent
	ActionExecutions []CaseActionExecution
}

// CountTemplateCasesForTargetParams groups the validated inputs needed for count template cases for target params.
type CountTemplateCasesForTargetParams struct {
	GuildID, TemplateID, TargetDiscordUserID string
}

// ListCasesParams groups the validated inputs needed for list cases params.
type ListCasesParams struct {
	GuildID, TargetDiscordUserID, ModeratorDiscordUserID, TemplateID string
	Validity                                                         CaseValidity
	Limit, Offset                                                    int
}

// ListCasesResult captures the outcome of list cases result for the caller.
type ListCasesResult struct {
	Cases []Case
	Total int64
}

// TargetCaseSummary groups the target case summary state used to keep this package's responsibilities explicit.
type TargetCaseSummary struct {
	Total      int64
	ByValidity map[CaseValidity]int64
	ByTemplate map[string]int64
}

// ClaimedCaseAction identifies the supported claimed case action values stored and exchanged by Quack.
type ClaimedCaseAction struct {
	Case      Case
	Execution CaseActionExecution
}

// ClaimCaseActionParams groups the validated inputs needed for claim case action params.
type ClaimCaseActionParams struct{ CaseID, WorkerID string }

// CompleteCaseActionParams groups the validated inputs needed for complete case action params.
type CompleteCaseActionParams struct {
	ExecutionID                                                      string
	AttemptNumber                                                    uint8
	WorkerID                                                         string
	AttemptStatus                                                    ActionAttemptStatus
	ExecutionStatus                                                  ActionExecutionStatus
	ErrorCode, ErrorMessage, RequestPayloadJSON, ResponsePayloadJSON string
	NextRetryAt                                                      *time.Time
	EventType                                                        CaseEventType
	EventBody, EventMetadataJSON, CorrelationID, RequestID           string
}

// SkipCaseActionsParams groups the validated inputs needed for skip case actions params.
type SkipCaseActionsParams struct {
	CaseID                           string
	AfterPosition                    int
	Reason, CorrelationID, RequestID string
}

// UpsertGuildParams groups the validated inputs needed for upsert guild params.
type UpsertGuildParams struct{ DiscordGuildID, Name, IconURL, OwnerDiscordUserID string }

// UpsertStaffMemberParams groups the validated inputs needed for upsert staff member params.
type UpsertStaffMemberParams struct {
	GuildID, DiscordUserID string
	LastSeenPermissionBits uint64
	LastKnownDisplayName   string
	LastActiveAt           time.Time
}

// ActionStatusCount groups the action status count state used to keep this package's responsibilities explicit.
type ActionStatusCount struct {
	Status ActionExecutionStatus
	Count  int64
}

// OldestActionExecution groups the oldest action execution state used to keep this package's responsibilities explicit.
type OldestActionExecution struct {
	ID, CaseID  string
	CaseNumber  uint64
	ActionType  ActionType
	Status      ActionExecutionStatus
	CreatedAt   time.Time
	NextRetryAt *time.Time
}

// RecentActionFailure groups the recent action failure state used to keep this package's responsibilities explicit.
type RecentActionFailure struct {
	ID, CaseID               string
	CaseNumber               uint64
	ActionType               ActionType
	Status                   ActionExecutionStatus
	LastErrorCode, LastError string
	UpdatedAt                time.Time
}

// ActionQueueSnapshot groups the action queue snapshot state used to keep this package's responsibilities explicit.
type ActionQueueSnapshot struct {
	StatusCounts         []ActionStatusCount
	OldestPendingOrRetry *OldestActionExecution
	RecentFailures       []RecentActionFailure
}
