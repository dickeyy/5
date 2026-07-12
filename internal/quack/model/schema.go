package model

import (
	"time"
)

// PermissionAction identifies the supported permission action values stored and exchanged by Quack.
type PermissionAction string

const (
	PermissionActionCaseCreate         PermissionAction = "case.create"
	PermissionActionCaseTemplateRead   PermissionAction = "case_template.read"
	PermissionActionCaseTemplateWrite  PermissionAction = "case_template.write"
	PermissionActionCaseTemplateDelete PermissionAction = "case_template.delete"
	PermissionActionAppealReview       PermissionAction = "appeal.review"
	PermissionActionTicketResolve      PermissionAction = "ticket.resolve"
	PermissionActionAuditRead          PermissionAction = "audit.read"
	PermissionActionGuildSettingsRead  PermissionAction = "guild_settings.read"
	PermissionActionGuildSettingsWrite PermissionAction = "guild_settings.write"
)

// CaseValidity identifies whether a case participates in escalation history.
type CaseValidity string

const (
	CaseValidityValid  CaseValidity = "valid"
	CaseValidityVoided CaseValidity = "voided"
)

// CaseSource identifies the supported case source values stored and exchanged by Quack.
type CaseSource string

const (
	CaseSourceDashboard CaseSource = "dashboard"
	CaseSourceDiscord   CaseSource = "discord"
	CaseSourceHoneypot  CaseSource = "honeypot"
	CaseSourceV4Import  CaseSource = "v4_import"
)

// ActionType identifies the supported action type values stored and exchanged by Quack.
type ActionType string

const (
	ActionSendDM      ActionType = "send_dm"
	ActionTimeoutUser ActionType = "timeout_user"
	ActionKickUser    ActionType = "kick_user"
	ActionBanUser     ActionType = "ban_user"
)

// NotificationType identifies the supported notification type values stored and exchanged by Quack.
type NotificationType string

const (
	NotificationWarning NotificationType = "warning"
	NotificationTimeout NotificationType = "timeout"
	NotificationKick    NotificationType = "kick"
	NotificationBan     NotificationType = "ban"
)

// ActionExecutionStatus identifies the supported action execution status values stored and exchanged by Quack.
type ActionExecutionStatus string

const (
	ActionExecutionPending   ActionExecutionStatus = "pending"
	ActionExecutionRunning   ActionExecutionStatus = "running"
	ActionExecutionSucceeded ActionExecutionStatus = "succeeded"
	ActionExecutionFailed    ActionExecutionStatus = "failed"
	ActionExecutionRetrying  ActionExecutionStatus = "retrying"
	ActionExecutionSkipped   ActionExecutionStatus = "skipped"
	ActionExecutionCancelled ActionExecutionStatus = "cancelled"
)

// ActionAttemptStatus identifies the supported action attempt status values stored and exchanged by Quack.
type ActionAttemptStatus string

const (
	ActionAttemptSucceeded ActionAttemptStatus = "succeeded"
	ActionAttemptFailed    ActionAttemptStatus = "failed"
)

// EventVisibility identifies the supported event visibility values stored and exchanged by Quack.
type EventVisibility string

const (
	EventVisibilityInternal EventVisibility = "internal"
	EventVisibilityStaff    EventVisibility = "staff"
	EventVisibilityPublic   EventVisibility = "public"
)

// CaseEventType identifies the supported case event type values stored and exchanged by Quack.
type CaseEventType string

const (
	CaseEventCreated         CaseEventType = "case_created"
	CaseEventActionQueued    CaseEventType = "action_queued"
	CaseEventActionSucceeded CaseEventType = "action_succeeded"
	CaseEventActionFailed    CaseEventType = "action_failed"
	CaseEventAppealCreated   CaseEventType = "appeal_created"
)

// AppealStatus identifies the supported appeal status values stored and exchanged by Quack.
type AppealStatus string

const (
	AppealStatusPending  AppealStatus = "pending"
	AppealStatusAccepted AppealStatus = "accepted"
	AppealStatusRejected AppealStatus = "rejected"
	AppealStatusClosed   AppealStatus = "closed"
)

// TicketStatus identifies the supported ticket status values stored and exchanged by Quack.
type TicketStatus string

const (
	TicketStatusOpen      TicketStatus = "open"
	TicketStatusResolved  TicketStatus = "resolved"
	TicketStatusCancelled TicketStatus = "cancelled"
)

// AuditSource identifies the supported audit source values stored and exchanged by Quack.
type AuditSource string

const (
	AuditSourceAPI     AuditSource = "api"
	AuditSourceWeb     AuditSource = "web"
	AuditSourceDiscord AuditSource = "discord"
	AuditSourceSystem  AuditSource = "system"
)

// AuditResult captures the outcome of audit result for the caller.
type AuditResult string

const (
	AuditResultSuccess AuditResult = "success"
	AuditResultFailure AuditResult = "failure"
	AuditResultDenied  AuditResult = "denied"
)

// ULIDModel represents the persistence-free domain state for a ulidmodel.
type ULIDModel struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Guild represents the persistence-free domain state for a guild.
type Guild struct {
	ULIDModel
	DiscordGuildID     string
	Name               string
	IconURL            string
	OwnerDiscordUserID string
	IsActive           bool
}

// GuildSettings contains guild-owned Quack configuration without embedding optional-module runtime state in the moderation core.
type GuildSettings struct {
	ULIDModel
	GuildID                           string
	AuditMirrorChannelDiscordID       string
	ManagedEvidenceChannelDiscordID   string
	NotificationIntroduction          string
	NotificationFooter                string
	TicketsEnabled                    bool
	GeneralLoggingEnabled             bool
	HoneypotEnabled                   bool
	StarterPolicyTemplateID           string
	StarterPolicyNoticePending        bool
	StarterPolicyNoticeAcknowledgedAt *time.Time
}

// StaffMember represents the persistence-free domain state for a staff member.
type StaffMember struct {
	ULIDModel
	GuildID                string
	DiscordUserID          string
	LastSeenPermissionBits uint64
	LastKnownDisplayName   string
	LastActiveAt           *time.Time
}

// CaseTemplate represents the persistence-free domain state for a case template.
type CaseTemplate struct {
	ULIDModel
	GuildID                string
	Slug                   string
	Name                   string
	Description            string
	ReasonTemplate         string
	Appealable             bool
	Version                uint
	CreatedByDiscordUserID string
	UpdatedByDiscordUserID string
	ArchivedAt             *time.Time
}

// CaseTemplateLevel represents the persistence-free domain state for a case template level.
type CaseTemplateLevel struct {
	ULIDModel
	TemplateID       string
	Position         int
	Name             string
	IsDefault        bool
	TriggerCaseCount int
	NotifyUser       bool
}

// CaseTemplateLevelAction identifies the supported case template level action values stored and exchanged by Quack.
type CaseTemplateLevelAction struct {
	ULIDModel
	LevelID    string
	ActionType ActionType
	ConfigJSON string
	MaxRetries uint8
}

// Case represents the persistence-free domain state for a case.
type Case struct {
	ULIDModel
	GuildID                 string
	CaseNumber              uint64
	TemplateID              *string
	TemplateVersion         uint
	TemplateSnapshotJSON    string
	TargetDiscordUserID     string
	ModeratorDiscordUserID  string
	Reason                  string
	Validity                CaseValidity `gorm:"column:status"`
	Source                  CaseSource
	CorrelationID           string
	ContextChannelDiscordID string
	ContextMessageDiscordID string
	ContextURL              string
	MetadataJSON            string
}

// CaseActionExecution represents the persistence-free domain state for a case action execution.
type CaseActionExecution struct {
	ULIDModel
	CaseID             string
	TemplateActionID   *string
	Position           int
	ActionType         ActionType
	Status             ActionExecutionStatus
	IdempotencyKey     string
	ConfigSnapshotJSON string
	NotifyUser         bool
	NotificationType   string
	AttemptCount       uint8
	MaxRetries         uint8
	RetryBackoffMS     int
	SafeForRetry       bool
	Irreversible       bool
	LastErrorCode      string
	LastError          string
	StartedAt          *time.Time
	FinishedAt         *time.Time
	NextRetryAt        *time.Time
	CorrelationID      string
}

// CaseActionAttempt represents the persistence-free domain state for a case action attempt.
type CaseActionAttempt struct {
	ULIDModel
	ExecutionID         string
	AttemptNumber       uint8
	Status              ActionAttemptStatus
	WorkerID            string
	StartedAt           time.Time
	FinishedAt          *time.Time
	DurationMS          int64
	ErrorCode           string
	ErrorMessage        string
	RequestPayloadJSON  string
	ResponsePayloadJSON string
}

// CaseEvent represents the persistence-free domain state for a case event.
type CaseEvent struct {
	ULIDModel
	CaseID             string
	GuildID            string
	EventType          CaseEventType
	ActorDiscordUserID string
	ActorType          string
	Visibility         EventVisibility
	Body               string
	MetadataJSON       string
}

// Appeal represents the persistence-free domain state for a appeal.
type Appeal struct {
	ULIDModel
	GuildID                 string
	CaseID                  *string
	TargetDiscordUserID     string
	Status                  AppealStatus
	Content                 string
	DecisionReason          string
	ReviewedByDiscordUserID string
	ReviewedAt              *time.Time
	ReviewMessageDiscordID  string
	MetadataJSON            string
}

// AppealEvent represents the persistence-free domain state for a appeal event.
type AppealEvent struct {
	ULIDModel
	AppealID           string
	GuildID            string
	EventType          string
	ActorDiscordUserID string
	Body               string
	MetadataJSON       string
}

// Ticket represents the persistence-free domain state for a ticket.
type Ticket struct {
	ULIDModel
	GuildID                 string
	OwnerDiscordUserID      string
	ThreadDiscordChannelID  string
	Status                  TicketStatus
	LogMessageDiscordID     string
	ResolvedByDiscordUserID string
	ResolvedAt              *time.Time
	TranscriptURL           string
	MetadataJSON            string
}

// TicketEvent represents the persistence-free domain state for a ticket event.
type TicketEvent struct {
	ULIDModel
	TicketID           string
	GuildID            string
	EventType          string
	ActorDiscordUserID string
	Body               string
	MetadataJSON       string
}

// AuditLogEntry represents the persistence-free domain state for a audit log entry.
type AuditLogEntry struct {
	ULIDModel
	GuildID             string
	ActorDiscordUserID  string
	ActorPermissionBits uint64
	Source              AuditSource
	Action              string
	ResourceType        string
	ResourceID          string
	Result              AuditResult
	FailureReason       string
	CorrelationID       string
	RequestID           string
	MetadataJSON        string
}
