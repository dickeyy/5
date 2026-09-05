package quack

import (
	"context"

	"github.com/quackdiscord/bot/internal/quack/model"
)

// audit records audit so moderation changes remain attributable.
func (s *CaseService) audit(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) error {
	return s.auditWithAttribution(ctx, guildContext, caseCreateAttribution{actorType: "staff", auditSource: model.AuditSourceAPI}, action, resourceType, resourceID, result, failureReason)
}

// auditWithAttribution appends case evidence without inventing a Discord actor
// for system automation.
func (s *CaseService) auditWithAttribution(ctx context.Context, guildContext *GuildStaffContext, attribution caseCreateAttribution, action, resourceType, resourceID string, result model.AuditResult, failureReason string) error {
	entry := s.auditEntryWithAttribution(ctx, guildContext, attribution, action, resourceType, resourceID, result, failureReason)
	if entry == nil {
		return nil
	}
	return recordAudit(ctx, s.store, entry)
}

// auditEntry records audit entry so moderation changes remain attributable.
func (s *CaseService) auditEntry(ctx context.Context, guildContext *GuildStaffContext, action, resourceType, resourceID string, result model.AuditResult, failureReason string) *model.AuditLogEntry {
	return s.auditEntryWithAttribution(ctx, guildContext, caseCreateAttribution{actorType: "staff", auditSource: model.AuditSourceAPI}, action, resourceType, resourceID, result, failureReason)
}

// auditEntryWithAttribution builds the atomic case audit row for either a
// current staff actor or Quack's restricted honeypot system actor.
func (s *CaseService) auditEntryWithAttribution(ctx context.Context, guildContext *GuildStaffContext, attribution caseCreateAttribution, action, resourceType, resourceID string, result model.AuditResult, failureReason string) *model.AuditLogEntry {
	if guildContext == nil || guildContext.Guild == nil || guildContext.Staff == nil {
		return nil
	}
	requestID, correlationID := TraceIDsFromContext(ctx)
	actorDiscordUserID := guildContext.Staff.DiscordUserID
	permissionBits := guildContext.PermissionBits
	if attribution.system {
		actorDiscordUserID = ""
		permissionBits = 0
	}

	entry := &model.AuditLogEntry{
		GuildID:             guildContext.Guild.ID,
		ActorDiscordUserID:  actorDiscordUserID,
		ActorPermissionBits: permissionBits,
		Source:              AuditSourceFromContext(ctx),
		Action:              action,
		ResourceType:        resourceType,
		ResourceID:          resourceID,
		Result:              result,
		FailureReason:       failureReason,
		CorrelationID:       correlationID,
		RequestID:           requestID,
		MetadataJSON:        "{}",
	}
	if entry.ResourceID == "" {
		entry.ResourceID = "unknown"
	}
	return entry
}

// ensureTraceContext encapsulates the ensure trace context rule so callers share one consistent package implementation.
func ensureTraceContext(ctx context.Context) context.Context {
	if RequestIDFromContext(ctx) != "" && CorrelationIDFromContext(ctx) != "" {
		return ctx
	}
	return ContextWithTrace(ctx, RequestIDFromContext(ctx), CorrelationIDFromContext(ctx))
}
