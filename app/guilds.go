package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/storage"
	"github.com/quackdiscord/bot/structs"
)

var (
	ErrUserNotInGuild = errors.New("user is not in guild")
	ErrBotNotInGuild  = errors.New("bot is not in guild")
)

type GuildService struct {
	store   *storage.Store
	discord DiscordClient
}

type GuildStaffContext struct {
	Guild              *structs.Guild
	Settings           *structs.GuildSettings
	Staff              *structs.StaffMember
	DiscordUserID      string
	DisplayName        string
	PermissionBits     uint64
	Permissions        map[structs.PermissionAction]bool
	IsOwner            bool
	IsAdministrator    bool
	IsModerator        bool
	IsMember           bool
	CanManageGuild     bool
	CanModerateMembers bool
}

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

type DiscordStaffContextInput struct {
	DiscordGuildID string
	DiscordUserID  string
	DisplayName    string
	PermissionBits uint64
	LastActiveAt   time.Time
}

func NewGuildService(store *storage.Store, discord DiscordClient) *GuildService {
	return &GuildService{store: store, discord: discord}
}

func (s *GuildService) ListUserManageableGuilds(ctx context.Context, session *structs.AuthSession) ([]UserGuildListItem, error) {
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
		isAdmin := hasAllBits(guild.Permissions, uint64(discordgo.PermissionAdministrator))
		canManageGuild := guild.Owner || isAdmin || hasAllBits(guild.Permissions, uint64(discordgo.PermissionManageGuild))
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

func (s *GuildService) ResolveStaffContext(ctx context.Context, session *structs.AuthSession, discordGuildID string) (*GuildStaffContext, error) {
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

	guild, err := s.store.UpsertGuild(ctx, storage.UpsertGuildParams{
		DiscordGuildID:     botGuild.ID,
		Name:               botGuild.Name,
		IconURL:            discordGuildIconURL(botGuild.ID, botGuild.Icon),
		OwnerDiscordUserID: botGuild.OwnerID,
	})
	if err != nil {
		return nil, err
	}

	settings, err := s.store.EnsureGuildSettings(ctx, guild.ID)
	if err != nil {
		return nil, err
	}

	staff, err := s.store.UpsertStaffMember(ctx, storage.UpsertStaffMemberParams{
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
	displayName := staffDisplayName(session)

	return &GuildStaffContext{
		Guild:              guild,
		Settings:           settings,
		Staff:              staff,
		DiscordUserID:      session.DiscordUserID,
		DisplayName:        displayName,
		PermissionBits:     userGuild.Permissions,
		Permissions:        role.permissions,
		IsOwner:            role.isOwner,
		IsAdministrator:    role.isAdministrator,
		IsModerator:        role.isModerator,
		IsMember:           role.isMember,
		CanManageGuild:     role.canManageGuild,
		CanModerateMembers: role.canModerateMembers,
	}, nil
}

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

	guild, err := s.store.UpsertGuild(ctx, storage.UpsertGuildParams{
		DiscordGuildID:     botGuild.ID,
		Name:               botGuild.Name,
		IconURL:            discordGuildIconURL(botGuild.ID, botGuild.Icon),
		OwnerDiscordUserID: botGuild.OwnerID,
	})
	if err != nil {
		return nil, err
	}

	settings, err := s.store.EnsureGuildSettings(ctx, guild.ID)
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

	staff, err := s.store.UpsertStaffMember(ctx, storage.UpsertStaffMemberParams{
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
		Guild:              guild,
		Settings:           settings,
		Staff:              staff,
		DiscordUserID:      discordUserID,
		DisplayName:        displayName,
		PermissionBits:     input.PermissionBits,
		Permissions:        role.permissions,
		IsOwner:            role.isOwner,
		IsAdministrator:    role.isAdministrator,
		IsModerator:        role.isModerator,
		IsMember:           role.isMember,
		CanManageGuild:     role.canManageGuild,
		CanModerateMembers: role.canModerateMembers,
	}, nil
}

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

func (ctx *GuildStaffContext) Can(action structs.PermissionAction) bool {
	if ctx == nil {
		return false
	}
	if ctx.Permissions == nil {
		return false
	}

	return ctx.Permissions[action]
}

type discordRole struct {
	isOwner            bool
	isAdministrator    bool
	isModerator        bool
	isMember           bool
	canManageGuild     bool
	canModerateMembers bool
	permissions        map[structs.PermissionAction]bool
}

func discordRoleContext(permissionBits uint64, isOwner bool) discordRole {
	isAdmin := hasAllBits(permissionBits, uint64(discordgo.PermissionAdministrator))
	hasManageGuild := hasAllBits(permissionBits, uint64(discordgo.PermissionManageGuild))
	hasModerateMembers := hasAllBits(permissionBits, uint64(discordgo.PermissionModerateMembers))

	role := discordRole{
		isOwner:            isOwner,
		isAdministrator:    isAdmin,
		isModerator:        !isOwner && !isAdmin && hasModerateMembers,
		canManageGuild:     isOwner || isAdmin || hasManageGuild,
		canModerateMembers: isOwner || isAdmin || hasModerateMembers,
	}
	role.isMember = !role.isOwner && !role.isAdministrator && !role.isModerator
	role.permissions = discordPermissionMap(role)
	return role
}

func discordPermissionMap(role discordRole) map[structs.PermissionAction]bool {
	canModerate := role.canModerateMembers
	canManage := role.canManageGuild

	return map[structs.PermissionAction]bool{
		structs.PermissionActionCaseCreate:         canModerate,
		structs.PermissionActionCaseTemplateRead:   canModerate,
		structs.PermissionActionCaseTemplateWrite:  canManage,
		structs.PermissionActionCaseTemplateDelete: canManage,
		structs.PermissionActionAppealReview:       canModerate,
		structs.PermissionActionTicketResolve:      canModerate,
		structs.PermissionActionAuditRead:          canManage,
	}
}

func hasAllBits(bits, required uint64) bool {
	if required == 0 {
		return true
	}

	return bits&required == required
}

func staffDisplayName(session *structs.AuthSession) string {
	if session == nil {
		return ""
	}
	if strings.TrimSpace(session.GlobalName) != "" {
		return session.GlobalName
	}
	return session.Username
}

func PermissionMapStrings(permissions map[structs.PermissionAction]bool) map[string]bool {
	out := make(map[string]bool, len(permissions))
	for action, allowed := range permissions {
		out[string(action)] = allowed
	}
	return out
}

func PermissionBitsString(bits uint64) string {
	return fmt.Sprintf("%d", bits)
}
