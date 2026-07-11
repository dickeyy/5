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
	ErrUserNotInGuild = errors.New("user is not in guild")
	ErrBotNotInGuild  = errors.New("bot is not in guild")
)

// GuildService resolves dashboard and Discord identities into a common authorized staff context.
type GuildService struct {
	store   Repository
	discord DiscordClient
}

// GuildStaffContext carries the request-scoped guild staff context data needed by downstream logic.
type GuildStaffContext struct {
	Guild          *model.Guild
	Staff          *model.StaffMember
	PermissionBits uint64
	Permissions    map[model.PermissionAction]bool
	IsAdmin        bool
	IsModerator    bool
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

// NewGuildService constructs guild service with required dependencies explicit so callers control lifecycle and substitution.
func NewGuildService(store Repository, discord DiscordClient) *GuildService {
	return &GuildService{store: store, discord: discord}
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
		if !canManageGuild {
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

	userGuild, err := s.userGuild(ctx, session.AccessToken, discordGuildID)
	if err != nil {
		return nil, err
	}

	botGuild, err := s.discord.BotGuild(ctx, discordGuildID)
	if err != nil {
		return nil, err
	}
	if botGuild == nil || botGuild.ID == "" {
		return nil, ErrBotNotInGuild
	}

	guild, err := s.store.UpsertGuild(ctx, model.UpsertGuildParams{
		DiscordGuildID:     botGuild.ID,
		Name:               botGuild.Name,
		IconURL:            discordGuildIconURL(botGuild.ID, botGuild.Icon),
		OwnerDiscordUserID: botGuild.OwnerID,
	})
	if err != nil {
		return nil, err
	}

	staff, err := s.store.UpsertStaffMember(ctx, model.UpsertStaffMemberParams{
		GuildID:                guild.ID,
		DiscordUserID:          session.DiscordUserID,
		LastSeenPermissionBits: userGuild.Permissions,
		LastKnownDisplayName:   staffDisplayName(session),
		LastActiveAt:           time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}

	isOwner := userGuild.Owner || guild.OwnerDiscordUserID == session.DiscordUserID
	role := discordRoleContext(userGuild.Permissions, isOwner)

	return &GuildStaffContext{
		Guild:          guild,
		Staff:          staff,
		PermissionBits: userGuild.Permissions,
		Permissions:    role.permissions,
		IsAdmin:        role.isAdmin,
		IsModerator:    role.isModerator,
	}, nil
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

	botGuild, err := s.discord.BotGuild(ctx, discordGuildID)
	if err != nil {
		return nil, err
	}
	if botGuild == nil || botGuild.ID == "" {
		return nil, ErrBotNotInGuild
	}

	guild, err := s.store.UpsertGuild(ctx, model.UpsertGuildParams{
		DiscordGuildID:     botGuild.ID,
		Name:               botGuild.Name,
		IconURL:            discordGuildIconURL(botGuild.ID, botGuild.Icon),
		OwnerDiscordUserID: botGuild.OwnerID,
	})
	if err != nil {
		return nil, err
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = discordUserID
	}
	lastActiveAt := input.LastActiveAt
	if lastActiveAt.IsZero() {
		lastActiveAt = time.Now().UTC()
	}

	staff, err := s.store.UpsertStaffMember(ctx, model.UpsertStaffMemberParams{
		GuildID:                guild.ID,
		DiscordUserID:          discordUserID,
		LastSeenPermissionBits: input.PermissionBits,
		LastKnownDisplayName:   displayName,
		LastActiveAt:           lastActiveAt,
	})
	if err != nil {
		return nil, err
	}

	isOwner := guild.OwnerDiscordUserID == discordUserID
	role := discordRoleContext(input.PermissionBits, isOwner)

	return &GuildStaffContext{
		Guild:          guild,
		Staff:          staff,
		PermissionBits: input.PermissionBits,
		Permissions:    role.permissions,
		IsAdmin:        role.isAdmin,
		IsModerator:    role.isModerator,
	}, nil
}

// userGuild encapsulates the user guild rule so callers share one consistent package implementation.
func (s *GuildService) userGuild(ctx context.Context, accessToken, discordGuildID string) (*DiscordUserGuild, error) {
	guilds, err := s.discord.UserGuilds(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	for i := range guilds {
		if guilds[i].ID == discordGuildID {
			return &guilds[i], nil
		}
	}

	return nil, ErrUserNotInGuild
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
		model.PermissionActionCaseTemplateRead:   canModerate,
		model.PermissionActionCaseTemplateWrite:  canManage,
		model.PermissionActionCaseTemplateDelete: canManage,
		model.PermissionActionAppealReview:       canModerate,
		model.PermissionActionTicketResolve:      canModerate,
		model.PermissionActionAuditRead:          canManage,
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
