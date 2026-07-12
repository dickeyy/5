package quack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack/model"
)

// GuildOperationalHealth reports guild-local Discord and managed-channel
// degradation without changing process readiness for healthy guilds.
type GuildOperationalHealth struct {
	Degraded        bool            `json:"degraded"`
	Reasons         []string        `json:"reasons"`
	BotPermissions  map[string]bool `json:"bot_permissions"`
	ManagedChannels map[string]bool `json:"managed_channels"`
}

// OperationalGuildHealth refreshes the bot's current guild permissions and
// combines them with managed-channel configuration. Deleted channel gateway
// reconciliation clears stale references, so missing required references are
// reported immediately and isolated to this guild.
func (s *GuildService) OperationalGuildHealth(ctx context.Context, discordGuildID string) (GuildOperationalHealth, error) {
	status := GuildOperationalHealth{Reasons: []string{}, BotPermissions: map[string]bool{}, ManagedChannels: map[string]bool{}}
	if s == nil || s.store == nil || s.discord == nil {
		return status, ErrAuthorizationUnavailable
	}
	guild, err := s.store.GetGuildByDiscordID(ctx, strings.TrimSpace(discordGuildID))
	if err != nil || guild == nil {
		return status, ErrBotNotInGuild
	}
	live, err := s.discord.GuildAuthorization(ctx, guild.DiscordGuildID, "", "")
	if err != nil || live == nil || !live.Bot.Present {
		status.Degraded = true
		status.Reasons = append(status.Reasons, "discord_bot_unavailable")
		return status, nil
	}
	permissions := map[string]int64{
		"moderate_members": discordgo.PermissionModerateMembers,
		"kick_members":     discordgo.PermissionKickMembers,
		"ban_members":      discordgo.PermissionBanMembers,
		"manage_channels":  discordgo.PermissionManageChannels,
	}
	for name, permission := range permissions {
		available := hasDiscordPermission(live.Bot.PermissionBits, uint64(permission))
		status.BotPermissions[name] = available
		if !available {
			status.Degraded = true
			status.Reasons = append(status.Reasons, "missing_bot_permission:"+name)
		}
	}
	settings, err := s.store.GetGuildSettings(ctx, guild.ID)
	if err != nil || settings == nil {
		return status, err
	}
	status.ManagedChannels["evidence"] = strings.TrimSpace(settings.ManagedEvidenceChannelDiscordID) != ""
	status.ManagedChannels["audit_mirror"] = strings.TrimSpace(settings.AuditMirrorChannelDiscordID) != ""
	if !status.ManagedChannels["evidence"] {
		status.Degraded = true
		status.Reasons = append(status.Reasons, "managed_evidence_channel_unavailable")
	}
	return status, nil
}

var (
	ErrBotNotInGuild = errors.New("bot is not in guild")
)

// GuildService resolves dashboard and Discord identities into a common authorized staff context.
type GuildService struct {
	store   Repository
	discord DiscordClient
}

// GuildStaffContext carries the request-scoped guild staff context data needed by downstream logic.
type GuildStaffContext struct {
	Guild              *model.Guild
	Staff              *model.StaffMember
	ActorDiscordUserID string
	PermissionBits     uint64
	Permissions        map[model.PermissionAction]bool
	IsAdmin            bool
	IsModerator        bool
	Live               DiscordGuildAuthorization
}

// UserGuildListItem groups the user guild list item state used to keep this package's responsibilities explicit.
type UserGuildListItem struct {
	DiscordGuildID  string `json:"discord_guild_id"`
	Name            string `json:"name"`
	IconURL         string `json:"icon_url"`
	PermissionBits  string `json:"permission_bits"`
	IsOwner         bool   `json:"is_owner"`
	IsAdministrator bool   `json:"is_administrator"`
	CanManageGuild  bool   `json:"can_manage_guild"`
	CanModerate     bool   `json:"can_moderate"`
	QuackInGuild    bool   `json:"quack_in_guild"`
	QuackGuildName  string `json:"quack_guild_name,omitempty"`
}

// DiscordStaffContextInput groups the validated inputs needed for discord staff context input.
type DiscordStaffContextInput struct {
	DiscordGuildID string
	DiscordUserID  string
	DisplayName    string
	PermissionBits uint64
	LastActiveAt   time.Time
}

// DiscordGuildLifecycleInput carries authoritative guild metadata and an optional complete channel inventory from a gateway event.
type DiscordGuildLifecycleInput struct {
	DiscordGuildID         string
	Name                   string
	Icon                   string
	OwnerDiscordUserID     string
	KnownChannelDiscordIDs []string
}

// NewGuildService constructs guild service with required dependencies explicit so callers control lifecycle and substitution.
func NewGuildService(store Repository, discord DiscordClient) *GuildService {
	return &GuildService{store: store, discord: discord}
}

// BootstrapDiscordGuild atomically installs or reactivates a guild, refreshes metadata, repairs stale channels, and ensures one starter policy.
func (s *GuildService) BootstrapDiscordGuild(ctx context.Context, input DiscordGuildLifecycleInput) (*model.BootstrapGuildResult, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("guild service is not configured")
	}
	guildID := strings.TrimSpace(input.DiscordGuildID)
	if guildID == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.OwnerDiscordUserID) == "" {
		return nil, errors.New("guild id, name, and owner are required")
	}
	return s.store.BootstrapGuild(ctx, model.BootstrapGuildParams{
		DiscordGuildID: guildID, Name: strings.TrimSpace(input.Name),
		IconURL:                discordGuildIconURL(guildID, strings.TrimSpace(input.Icon)),
		OwnerDiscordUserID:     strings.TrimSpace(input.OwnerDiscordUserID),
		KnownChannelDiscordIDs: input.KnownChannelDiscordIDs,
	})
}

// DeactivateDiscordGuild marks a true guild departure inactive while preserving settings, starter policy, and history.
func (s *GuildService) DeactivateDiscordGuild(ctx context.Context, discordGuildID string) (*model.Guild, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("guild service is not configured")
	}
	return s.store.DeactivateGuild(ctx, strings.TrimSpace(discordGuildID), systemGuildAudit("guild.lifecycle.leave", "guild"))
}

// ClearDeletedChannel removes stale core settings references when Discord confirms a configured channel was deleted.
func (s *GuildService) ClearDeletedChannel(ctx context.Context, discordGuildID, channelID string) (*model.GuildSettings, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("guild service is not configured")
	}
	guild, err := s.store.GetGuildByDiscordID(ctx, strings.TrimSpace(discordGuildID))
	if err != nil || guild == nil {
		return nil, err
	}
	return s.store.ClearGuildChannelReferences(ctx, guild.ID, strings.TrimSpace(channelID), systemGuildAudit("guild_settings.channel_reference.cleared", "guild_settings"))
}

// systemGuildAudit creates adapter-attributed lifecycle evidence without pretending the bot is a Discord staff member.
func systemGuildAudit(action, resourceType string) *model.AuditLogEntry {
	return &model.AuditLogEntry{
		ActorDiscordUserID: "quack-system", Source: model.AuditSourceDiscord,
		Action: action, ResourceType: resourceType, Result: model.AuditResultSuccess,
		MetadataJSON: "{}",
	}
}

// ListUserManageableGuilds returns user manageable guilds subject to authorization, ordering, and filtering constraints.
func (s *GuildService) ListUserManageableGuilds(ctx context.Context, session *model.AuthSession) ([]UserGuildListItem, error) {
	if s == nil || s.discord == nil {
		return nil, errors.New("guild service is not configured")
	}
	if session == nil || session.AccessToken == "" {
		return nil, errors.New("missing auth session")
	}

	userGuilds, err := s.discord.UserGuilds(ctx, session.AccessToken)
	if err != nil {
		return nil, err
	}

	botGuilds, err := s.discord.BotGuilds(ctx)
	if err != nil {
		return nil, err
	}

	botGuildsByID := make(map[string]DiscordBotGuild, len(botGuilds))
	for _, guild := range botGuilds {
		botGuildsByID[guild.ID] = guild
	}

	out := make([]UserGuildListItem, 0, len(userGuilds))
	for _, guild := range userGuilds {
		isAdmin := hasAllBits(guild.Permissions, permissionAdministrator)
		canManageGuild := guild.Owner || isAdmin || hasAllBits(guild.Permissions, permissionManageGuild)
		canModerate := guild.Owner || isAdmin || hasAllBits(guild.Permissions, permissionModerateMembers)
		if !canManageGuild && !canModerate {
			continue
		}

		item := UserGuildListItem{
			DiscordGuildID:  guild.ID,
			Name:            guild.Name,
			IconURL:         discordGuildIconURL(guild.ID, guild.Icon),
			PermissionBits:  PermissionBitsString(guild.Permissions),
			IsOwner:         guild.Owner,
			IsAdministrator: isAdmin,
			CanManageGuild:  canManageGuild,
			CanModerate:     canModerate,
		}

		if botGuild, ok := botGuildsByID[guild.ID]; ok {
			item.QuackInGuild = true
			item.QuackGuildName = botGuild.Name
		}

		out = append(out, item)
	}

	return out, nil
}

// ResolveStaffContext resolves staff context from authoritative request and repository data.
func (s *GuildService) ResolveStaffContext(ctx context.Context, session *model.AuthSession, discordGuildID string) (*GuildStaffContext, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("guild service is not configured")
	}
	if s.discord == nil {
		return nil, errors.New("discord client is not configured")
	}
	if session == nil || session.DiscordUserID == "" {
		return nil, errors.New("missing auth session")
	}

	discordGuildID = strings.TrimSpace(discordGuildID)
	if discordGuildID == "" {
		return nil, errors.New("missing discord guild id")
	}

	snapshot, err := s.discord.GuildAuthorization(ctx, discordGuildID, session.DiscordUserID, "")
	if err != nil || snapshot == nil {
		if errors.Is(err, ErrBotNotInGuild) {
			return nil, ErrBotNotInGuild
		}
		return nil, ErrAuthorizationUnavailable
	}
	if snapshot.Guild.ID != discordGuildID {
		return nil, ErrAuthorizationUnavailable
	}
	return s.contextFromAuthorization(ctx, snapshot, session.DiscordUserID, staffDisplayName(session))
}

// ResolveDiscordStaffContext resolves discord staff context from authoritative request and repository data.
func (s *GuildService) ResolveDiscordStaffContext(ctx context.Context, input DiscordStaffContextInput) (*GuildStaffContext, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("guild service is not configured")
	}
	if s.discord == nil {
		return nil, errors.New("discord client is not configured")
	}

	discordGuildID := strings.TrimSpace(input.DiscordGuildID)
	if discordGuildID == "" {
		return nil, errors.New("missing discord guild id")
	}
	discordUserID := strings.TrimSpace(input.DiscordUserID)
	if discordUserID == "" {
		return nil, errors.New("missing discord user id")
	}

	snapshot, err := s.discord.GuildAuthorization(ctx, discordGuildID, discordUserID, "")
	if err != nil || snapshot == nil {
		if errors.Is(err, ErrBotNotInGuild) {
			return nil, ErrBotNotInGuild
		}
		return nil, ErrAuthorizationUnavailable
	}
	if snapshot.Guild.ID != discordGuildID {
		return nil, ErrAuthorizationUnavailable
	}
	return s.contextFromAuthorization(ctx, snapshot, discordUserID, input.DisplayName)
}

// contextFromAuthorization materializes a request context from live Discord state and refreshes attribution cache data only after successful resolution.
func (s *GuildService) contextFromAuthorization(ctx context.Context, snapshot *DiscordGuildAuthorization, actorDiscordUserID, fallbackDisplayName string) (*GuildStaffContext, error) {
	if snapshot == nil || snapshot.Guild.ID == "" || snapshot.Guild.ID != strings.TrimSpace(snapshot.Guild.ID) {
		return nil, ErrAuthorizationUnavailable
	}
	if snapshot.Actor.DiscordUserID != actorDiscordUserID {
		return nil, ErrAuthorizationUnavailable
	}
	guild, err := s.store.UpsertGuild(ctx, model.UpsertGuildParams{
		DiscordGuildID: snapshot.Guild.ID, Name: snapshot.Guild.Name,
		IconURL: discordGuildIconURL(snapshot.Guild.ID, snapshot.Guild.Icon), OwnerDiscordUserID: snapshot.Guild.OwnerID,
	})
	if err != nil {
		return nil, err
	}

	var staff *model.StaffMember
	if snapshot.Actor.Present {
		displayName := strings.TrimSpace(snapshot.Actor.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(fallbackDisplayName)
		}
		if displayName == "" {
			displayName = actorDiscordUserID
		}
		staff, err = s.store.UpsertStaffMember(ctx, model.UpsertStaffMemberParams{
			GuildID: guild.ID, DiscordUserID: actorDiscordUserID,
			LastSeenPermissionBits: snapshot.Actor.PermissionBits,
			LastKnownDisplayName:   displayName, LastActiveAt: authorizationNow(),
		})
	} else {
		staff, err = s.store.GetStaffMember(ctx, guild.ID, actorDiscordUserID)
	}
	if err != nil {
		return nil, err
	}

	isOwner := snapshot.Guild.OwnerID == actorDiscordUserID
	role := discordRoleContext(snapshot.Actor.PermissionBits, isOwner)
	return &GuildStaffContext{
		Guild: guild, Staff: staff, ActorDiscordUserID: actorDiscordUserID,
		PermissionBits: snapshot.Actor.PermissionBits, Permissions: role.permissions,
		IsAdmin: role.isAdmin, IsModerator: role.isModerator, Live: *snapshot,
	}, nil
}

// Can reports whether the staff context grants every requested moderation permission.
func (ctx *GuildStaffContext) Can(action model.PermissionAction) bool {
	if ctx == nil {
		return false
	}
	if ctx.Permissions == nil {
		return false
	}

	return ctx.Permissions[action]
}

// discordRole groups the discord role state used to keep this package's responsibilities explicit.
type discordRole struct {
	isAdmin     bool
	isModerator bool
	permissions map[model.PermissionAction]bool
}

// discordRoleContext encapsulates the discord role context rule so callers share one consistent package implementation.
func discordRoleContext(permissionBits uint64, isOwner bool) discordRole {
	isAdmin := isOwner || hasAllBits(permissionBits, permissionAdministrator)
	hasManageGuild := hasAllBits(permissionBits, permissionManageGuild)
	hasModerateMembers := hasAllBits(permissionBits, permissionModerateMembers)
	canManage := isAdmin || hasManageGuild
	canModerate := isAdmin || hasModerateMembers

	role := discordRole{
		isAdmin:     isAdmin,
		isModerator: !isAdmin && hasModerateMembers,
	}
	role.permissions = discordPermissionMap(canModerate, canManage)
	return role
}

// discordPermissionMap encapsulates the discord permission map rule so callers share one consistent package implementation.
func discordPermissionMap(canModerate, canManage bool) map[model.PermissionAction]bool {
	return map[model.PermissionAction]bool{
		model.PermissionActionCaseCreate:         canModerate,
		model.PermissionActionCaseRead:           canModerate,
		model.PermissionActionCaseTemplateRead:   canModerate || canManage,
		model.PermissionActionCaseTemplateWrite:  canManage,
		model.PermissionActionCaseTemplateDelete: canManage,
		model.PermissionActionAppealReview:       canModerate,
		model.PermissionActionTicketResolve:      canModerate,
		model.PermissionActionAuditRead:          canModerate,
		model.PermissionActionGuildSettingsRead:  canManage,
		model.PermissionActionGuildSettingsWrite: canManage,
		model.PermissionActionCaseVoid:           canModerate,
		model.PermissionActionFailureDismiss:     canModerate,
	}
}

// hasAllBits encapsulates the has all bits rule so callers share one consistent package implementation.
func hasAllBits(bits, required uint64) bool {
	if required == 0 {
		return true
	}

	return bits&required == required
}

// staffDisplayName encapsulates the staff display name rule so callers share one consistent package implementation.
func staffDisplayName(session *model.AuthSession) string {
	if session == nil {
		return ""
	}
	if strings.TrimSpace(session.GlobalName) != "" {
		return session.GlobalName
	}
	return session.Username
}

// PermissionMapStrings encapsulates the permission map strings rule so callers share one consistent package implementation.
func PermissionMapStrings(permissions map[model.PermissionAction]bool) map[string]bool {
	out := make(map[string]bool, len(permissions))
	for action, allowed := range permissions {
		out[string(action)] = allowed
	}
	return out
}

// PermissionBitsString encapsulates the permission bits string rule so callers share one consistent package implementation.
func PermissionBitsString(bits uint64) string {
	return fmt.Sprintf("%d", bits)
}
