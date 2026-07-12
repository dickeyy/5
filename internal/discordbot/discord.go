package discordbot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/actionmods"
)

const discordUserGuildsURL = "https://discord.com/api/v10/users/@me/guilds"

// Bot owns the Discord session and adapts Discord guild and messaging operations to core ports.
type Bot struct {
	Session    *discordgo.Session
	HTTPClient *http.Client
}

// New constructs new with required dependencies explicit so callers control lifecycle and substitution.
func New(token string) (*Bot, error) {
	session, err := discordgo.New(token)
	if err != nil {
		return nil, err
	}
	session.Identify.Intents = discordgo.Intent(3276543)
	session.StateEnabled = true
	session.State.MaxMessageCount = 5000
	return &Bot{Session: session, HTTPClient: http.DefaultClient}, nil
}

// Open opens and verifies open so startup fails before serving traffic when the dependency is unavailable.
func (b *Bot) Open() error {
	if b == nil || b.Session == nil {
		return errors.New("discord session is not configured")
	}
	return b.Session.Open()
}

// Close releases resources owned by bot and is safe to use during reverse-order shutdown.
func (b *Bot) Close() error {
	if b == nil || b.Session == nil {
		return nil
	}
	return b.Session.Close()
}

// Status reports whether the adapter's external dependency is currently ready for health checks.
func (b *Bot) Status() (bool, string, int64) {
	if b == nil || b.Session == nil || b.Session.State == nil || b.Session.State.User == nil {
		return false, "", 0
	}
	return true, b.Session.State.User.Username, b.Session.HeartbeatLatency().Milliseconds()
}

// UserGuilds encapsulates the user guilds rule so callers share one consistent package implementation.
func (b *Bot) UserGuilds(ctx context.Context, accessToken string) ([]quack.DiscordUserGuild, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, errors.New("missing discord access token")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discordUserGuildsURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	client := b.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("discord user guilds failed with status %d", response.StatusCode)
	}
	var guilds []quack.DiscordUserGuild
	if err := json.NewDecoder(response.Body).Decode(&guilds); err != nil {
		return nil, err
	}
	return guilds, nil
}

// BotGuild encapsulates the bot guild rule so callers share one consistent package implementation.
func (b *Bot) BotGuild(ctx context.Context, guildID string) (*quack.DiscordBotGuild, error) {
	if b == nil || b.Session == nil {
		return nil, quack.ErrBotNotInGuild
	}
	if b.Session.State != nil {
		if guild, err := b.Session.State.Guild(guildID); err == nil && guild != nil {
			return botGuild(guild), nil
		}
	}
	guild, err := b.Session.Guild(guildID)
	if err != nil {
		return nil, quack.ErrBotNotInGuild
	}
	return botGuild(guild), nil
}

// BotGuilds encapsulates the bot guilds rule so callers share one consistent package implementation.
func (b *Bot) BotGuilds(context.Context) ([]quack.DiscordBotGuild, error) {
	if b == nil || b.Session == nil || b.Session.State == nil {
		return []quack.DiscordBotGuild{}, nil
	}
	b.Session.State.RLock()
	defer b.Session.State.RUnlock()
	guilds := make([]quack.DiscordBotGuild, 0, len(b.Session.State.Guilds))
	for _, guild := range b.Session.State.Guilds {
		if guild != nil {
			guilds = append(guilds, *botGuild(guild))
		}
	}
	return guilds, nil
}

// GuildAuthorization fetches current guild, actor, bot, and optional target state directly from Discord for one protected request.
func (b *Bot) GuildAuthorization(ctx context.Context, guildID, actorDiscordUserID, targetDiscordUserID string) (*quack.DiscordGuildAuthorization, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.Session == nil {
		return nil, quack.ErrAuthorizationUnavailable
	}
	guild, err := b.Session.Guild(guildID)
	if err != nil {
		return nil, guildAuthorizationError(err)
	}
	if guild == nil {
		return nil, quack.ErrAuthorizationUnavailable
	}

	botID := ""
	if b.Session.State != nil && b.Session.State.User != nil {
		botID = b.Session.State.User.ID
	}
	if botID == "" {
		botUser, userErr := b.Session.User("@me")
		if userErr != nil || botUser == nil {
			return nil, quack.ErrAuthorizationUnavailable
		}
		botID = botUser.ID
	}

	actor, err := b.liveMemberAuthorization(ctx, guild, actorDiscordUserID)
	if err != nil {
		return nil, err
	}
	bot, err := b.liveMemberAuthorization(ctx, guild, botID)
	if err != nil {
		return nil, err
	}
	snapshot := &quack.DiscordGuildAuthorization{Guild: *botGuild(guild), Actor: actor, Bot: bot}
	if strings.TrimSpace(targetDiscordUserID) != "" {
		target, targetErr := b.liveMemberAuthorization(ctx, guild, targetDiscordUserID)
		if targetErr != nil {
			return nil, targetErr
		}
		snapshot.Target = &target
	}
	return snapshot, nil
}

// guildAuthorizationError preserves inactive-guild semantics while hiding transient Discord failures.
func guildAuthorizationError(err error) error {
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil {
		switch restErr.Response.StatusCode {
		case http.StatusForbidden, http.StatusNotFound:
			return quack.ErrBotNotInGuild
		}
	}
	return quack.ErrAuthorizationUnavailable
}

// liveMemberAuthorization fetches one current member and safely represents Discord's unknown-member response as non-membership.
func (b *Bot) liveMemberAuthorization(ctx context.Context, guild *discordgo.Guild, userID string) (quack.DiscordMemberAuthorization, error) {
	memberState := quack.DiscordMemberAuthorization{DiscordUserID: strings.TrimSpace(userID)}
	if memberState.DiscordUserID == "" {
		return memberState, nil
	}
	if err := ctx.Err(); err != nil {
		return memberState, err
	}
	member, err := b.Session.GuildMember(guild.ID, memberState.DiscordUserID)
	if err != nil {
		var restErr *discordgo.RESTError
		if errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound {
			return memberState, nil
		}
		return memberState, quack.ErrAuthorizationUnavailable
	}
	return discordMemberAuthorization(guild, member), nil
}

// discordMemberAuthorization calculates guild-level permission and hierarchy state from a fresh guild/member response.
func discordMemberAuthorization(guild *discordgo.Guild, member *discordgo.Member) quack.DiscordMemberAuthorization {
	if guild == nil || member == nil || member.User == nil {
		return quack.DiscordMemberAuthorization{}
	}
	permissions := int64(0)
	topRolePosition := 0
	roleIDs := make(map[string]struct{}, len(member.Roles))
	for _, roleID := range member.Roles {
		roleIDs[roleID] = struct{}{}
	}
	for _, role := range guild.Roles {
		if role == nil {
			continue
		}
		if role.ID == guild.ID {
			permissions |= role.Permissions
		}
		if _, ok := roleIDs[role.ID]; ok {
			permissions |= role.Permissions
			if role.Position > topRolePosition {
				topRolePosition = role.Position
			}
		}
	}
	if member.User.ID == guild.OwnerID || permissions&discordgo.PermissionAdministrator != 0 {
		permissions |= discordgo.PermissionAll
	}
	displayName := strings.TrimSpace(member.Nick)
	if displayName == "" {
		displayName = strings.TrimSpace(member.User.GlobalName)
	}
	if displayName == "" {
		displayName = member.User.Username
	}
	return quack.DiscordMemberAuthorization{
		DiscordUserID: member.User.ID, DisplayName: displayName,
		PermissionBits: uint64(permissions), TopRolePosition: topRolePosition,
		Present: true, Bot: member.User.Bot,
	}
}

// botGuild encapsulates the bot guild rule so callers share one consistent package implementation.
func botGuild(guild *discordgo.Guild) *quack.DiscordBotGuild {
	return &quack.DiscordBotGuild{ID: guild.ID, Name: guild.Name, Icon: guild.Icon, OwnerID: guild.OwnerID}
}

// SendDM sends dm through the configured external gateway.
func (b *Bot) SendDM(ctx context.Context, userID, message string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b == nil || b.Session == nil {
		return nil, actionmods.DiscordError{Code: "discord_session_unavailable", Message: "discord session is unavailable", Retryable: true}
	}
	channel, err := b.Session.UserChannelCreate(userID)
	if err != nil {
		return nil, classifyDiscordError("send_dm_channel", err)
	}
	sent, err := b.Session.ChannelMessageSend(channel.ID, message)
	if err != nil {
		return nil, classifyDiscordError("send_dm_message", err)
	}
	result := map[string]any{"channel_id": channel.ID}
	if sent != nil {
		result["message_id"] = sent.ID
	}
	return result, nil
}

// PrepareDM opens the target's direct-message channel before an irreversible membership action.
func (b *Bot) PrepareDM(ctx context.Context, userID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if b == nil || b.Session == nil {
		return "", actionmods.DiscordError{Code: "discord_session_unavailable", Message: "Discord is unavailable", Retryable: true}
	}
	channel, err := b.Session.UserChannelCreate(userID)
	if err != nil {
		return "", classifyDiscordOperation("dm_prepare", err, false)
	}
	return channel.ID, nil
}

// SendPreparedDM sends one final structured case notification through a pre-opened channel.
func (b *Bot) SendPreparedDM(ctx context.Context, channelID, message string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sent, err := b.Session.ChannelMessageSend(channelID, message)
	if err != nil {
		return nil, classifyDiscordOperation("dm_send", err, true)
	}
	result := map[string]any{"channel_id": channelID}
	if sent != nil {
		result["message_id"] = sent.ID
	}
	return result, nil
}

// TimeoutMember applies the exact template-defined timeout duration.
func (b *Bot) TimeoutMember(ctx context.Context, guildID, userID string, durationSeconds int, auditReason string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	until := time.Now().UTC().Add(time.Duration(durationSeconds) * time.Second)
	if err := b.Session.GuildMemberTimeout(guildID, userID, &until, discordgo.WithAuditLogReason(auditReason)); err != nil {
		return nil, classifyDiscordOperation("timeout", err, false)
	}
	return map[string]any{"timeout_until": until.Format(time.RFC3339)}, nil
}

// KickMember removes the immutable case target using a bounded audit reason.
func (b *Bot) KickMember(ctx context.Context, guildID, userID, auditReason string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.Session.GuildMemberDeleteWithReason(guildID, userID, auditReason); err != nil {
		return nil, classifyDiscordOperation("kick", err, true)
	}
	return map[string]any{"result": "kicked"}, nil
}

// BanMember uses Discord's seconds-based deletion setting without rounding.
func (b *Bot) BanMember(ctx context.Context, guildID, userID string, deleteMessageSeconds int, auditReason string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	endpoint := discordgo.EndpointGuildBan(guildID, userID)
	_, err := b.Session.RequestWithBucketID(http.MethodPut, endpoint, map[string]any{"delete_message_seconds": deleteMessageSeconds}, discordgo.EndpointGuildBan(guildID, ""), discordgo.WithAuditLogReason(auditReason))
	if err != nil {
		return nil, classifyDiscordOperation("ban", err, true)
	}
	return map[string]any{"result": "banned", "delete_message_seconds": deleteMessageSeconds}, nil
}

// RemoveMemberTimeout executes an explicit staff-confirmed timeout reversal.
func (b *Bot) RemoveMemberTimeout(ctx context.Context, guildID, userID, auditReason string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.Session.GuildMemberTimeout(guildID, userID, nil, discordgo.WithAuditLogReason(auditReason)); err != nil {
		return nil, classifyDiscordOperation("remove_timeout", err, true)
	}
	return map[string]any{"result": "timeout_removed"}, nil
}

// UnbanMember executes an explicit staff-confirmed ban reversal.
func (b *Bot) UnbanMember(ctx context.Context, guildID, userID, auditReason string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := b.Session.GuildBanDelete(guildID, userID, discordgo.WithAuditLogReason(auditReason)); err != nil {
		return nil, classifyDiscordOperation("unban", err, true)
	}
	return map[string]any{"result": "unbanned"}, nil
}

// FetchMessageEvidence fetches a live message and maps it into the bounded core capture contract.
func (b *Bot) FetchMessageEvidence(ctx context.Context, ref quack.DiscordMessageReference) (*quack.DiscordMessageSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	message, err := b.Session.ChannelMessage(ref.ChannelID, ref.MessageID)
	if err != nil {
		var restErr *discordgo.RESTError
		if errors.As(err, &restErr) && restErr.Response != nil {
			if restErr.Response.StatusCode == http.StatusNotFound {
				return nil, &quack.EvidenceUnavailableError{Outcome: "deleted", Message: "linked message was deleted or does not exist"}
			}
			if restErr.Response.StatusCode == http.StatusForbidden {
				return nil, &quack.EvidenceUnavailableError{Outcome: "inaccessible", Message: "Quack cannot access the linked message"}
			}
		}
		return nil, classifyDiscordOperation("evidence_fetch", err, false)
	}
	if message == nil || message.Author == nil {
		return nil, &quack.EvidenceUnavailableError{Outcome: "unavailable", Message: "linked message is unavailable"}
	}
	if message.GuildID == "" {
		channel, channelErr := b.Session.Channel(ref.ChannelID)
		if channelErr != nil {
			return nil, classifyDiscordOperation("evidence_channel", channelErr, false)
		}
		message.GuildID = channel.GuildID
	}
	embeds := make([]map[string]any, 0, len(message.Embeds))
	for _, embed := range message.Embeds {
		body, _ := json.Marshal(embed)
		var value map[string]any
		_ = json.Unmarshal(body, &value)
		embeds = append(embeds, value)
	}
	attachments := make([]quack.DiscordAttachmentSnapshot, 0, len(message.Attachments))
	for _, item := range message.Attachments {
		attachments = append(attachments, quack.DiscordAttachmentSnapshot{ID: item.ID, Filename: item.Filename, ContentType: item.ContentType, SizeBytes: int64(item.Size), URL: item.URL})
	}
	return &quack.DiscordMessageSnapshot{GuildID: message.GuildID, ChannelID: message.ChannelID, MessageID: message.ID, AuthorDiscordUserID: message.Author.ID, URL: ref.URL, Content: message.Content, CreatedAt: message.Timestamp, EditedAt: message.EditedTimestamp, Embeds: embeds, Attachments: attachments}, nil
}

// PreserveEvidenceAttachment copies supported bytes into the guild's managed staff-only evidence channel.
func (b *Bot) PreserveEvidenceAttachment(ctx context.Context, guildID, channelID string, item quack.DiscordAttachmentSnapshot) (*quack.PreservedDiscordAttachment, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return nil, err
	}
	client := b.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, classifyDiscordOperation("evidence_download", err, false)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, actionmods.DiscordError{Code: "evidence_download_failed", Message: "attachment download failed", Retryable: response.StatusCode >= 500}
	}
	sent, err := b.Session.ChannelFileSend(channelID, item.Filename, io.LimitReader(response.Body, item.SizeBytes+1))
	if err != nil {
		return nil, classifyDiscordOperation("evidence_upload", err, false)
	}
	preserved := &quack.PreservedDiscordAttachment{}
	if sent != nil {
		preserved.MessageID = sent.ID
		if len(sent.Attachments) > 0 {
			preserved.AttachmentID = sent.Attachments[0].ID
			preserved.URL = sent.Attachments[0].URL
		}
	}
	return preserved, nil
}

// EnsureEvidenceChannel creates or repairs a staff-only evidence channel owned by Quack.
func (b *Bot) EnsureEvidenceChannel(ctx context.Context, guildID, currentChannelID string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	botID := ""
	if b.Session.State != nil && b.Session.State.User != nil {
		botID = b.Session.State.User.ID
	}
	if botID == "" {
		user, err := b.Session.User("@me")
		if err != nil || user == nil {
			return "", errors.New("Discord bot identity is unavailable")
		}
		botID = user.ID
	}
	overwrites := []*discordgo.PermissionOverwrite{{ID: guildID, Type: discordgo.PermissionOverwriteTypeRole, Deny: discordgo.PermissionViewChannel}, {ID: botID, Type: discordgo.PermissionOverwriteTypeMember, Allow: discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionAttachFiles | discordgo.PermissionReadMessageHistory}}
	if currentChannelID != "" {
		channel, err := b.Session.Channel(currentChannelID)
		if err == nil && channel != nil && channel.GuildID == guildID {
			_, editErr := b.Session.ChannelEditComplex(channel.ID, &discordgo.ChannelEdit{Name: "quack-evidence", Topic: "Quack-managed immutable moderation evidence", PermissionOverwrites: overwrites})
			if editErr != nil {
				return "", classifyDiscordOperation("evidence_channel_repair", editErr, false)
			}
			return channel.ID, nil
		}
	}
	created, err := b.Session.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{Name: "quack-evidence", Type: discordgo.ChannelTypeGuildText, Topic: "Quack-managed immutable moderation evidence", PermissionOverwrites: overwrites})
	if err != nil {
		return "", classifyDiscordOperation("evidence_channel_create", err, false)
	}
	return created.ID, nil
}

// classifyDiscordError encapsulates the classify discord error rule so callers share one consistent package implementation.
func classifyDiscordError(code string, err error) error {
	return classifyDiscordOperation(code, err, false)
}

// classifyDiscordOperation produces redacted retry and ambiguity semantics for persisted attempts.
func classifyDiscordOperation(operation string, err error, irreversible bool) error {
	var restError *discordgo.RESTError
	if errors.As(err, &restError) && restError.Response != nil {
		status := restError.Response.StatusCode
		code := "discord_failure"
		retryable := false
		uncertain := false
		switch {
		case status == http.StatusBadRequest:
			code = "validation_failed"
		case status == http.StatusUnauthorized || status == http.StatusForbidden:
			code = "permission_or_hierarchy_denied"
		case status == http.StatusNotFound:
			code = "unknown_member_or_resource"
		case status == http.StatusTooManyRequests:
			code = "rate_limited"
			retryable = true
		case status >= 500:
			code = "discord_server_error"
			retryable = !irreversible
			uncertain = irreversible
		}
		return actionmods.DiscordError{Code: operation + "_" + code, Message: "Discord rejected the moderation request", Retryable: retryable, OutcomeUncertain: uncertain}
	}
	return actionmods.DiscordError{Code: operation + "_network_error", Message: "Discord request failed", Retryable: !irreversible, OutcomeUncertain: irreversible}
}
