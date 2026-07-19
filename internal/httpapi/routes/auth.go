package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/apierror"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	httpplatform "github.com/quackdiscord/bot/internal/httpapi/platform"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/idutil"
	"github.com/quackdiscord/bot/internal/quack/model"
	"github.com/rs/zerolog/log"
)

const (
	discordAuthorizeURL = "https://discord.com/oauth2/authorize"
	maxDiscordBodyBytes = 1 << 20
)

var (
	// discordTokenEndpoint and discordMeEndpoint remain replaceable only for bounded adapter contract tests.
	discordTokenEndpoint = "https://discord.com/api/v10/oauth2/token"
	discordMeEndpoint    = "https://discord.com/api/v10/users/@me"
	discordHTTPClient    = &http.Client{Timeout: 10 * time.Second}
)

// userSessionRevoker is the optional auth-store contract used for compromise and account-change revocation.
type userSessionRevoker interface {
	RevokeUserSessions(context.Context, string) error
}

// discordTokenResponse is the transport-neutral representation returned for discord token response.
type discordTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
}

// discordUserResponse is the transport-neutral representation returned for discord user response.
type discordUserResponse struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	GlobalName string `json:"global_name"`
	Avatar     string `json:"avatar"`
}

// setupAuthRoutes explicitly wires setup auth routes so runtime behavior does not depend on init-time registration.
func setupAuthRoutes(r *gin.Engine, services *quack.Services) {
	auth := r.Group("/auth")
	primitives := httpplatform.FromRepository(services.Store)
	oauthLimit := httpplatform.RateLimit{
		Maximum: services.Config.RateLimits.OAuth.Maximum,
		Window:  time.Duration(services.Config.RateLimits.OAuth.WindowSeconds) * time.Second,
	}
	{
		auth.GET("/discord/login", primitives.RateLimits.Limit("oauth-login", oauthLimit, httpplatform.ClientIPSubject), func(c *gin.Context) { discordLogin(c, services) })
		auth.GET("/discord/callback", primitives.RateLimits.Limit("oauth-callback", oauthLimit, httpplatform.ClientIPSubject), func(c *gin.Context) { discordCallback(c, services) })

		protected := auth.Group("")
		protected.Use(middleware.RequireAuth(services.Store, services.Config.Auth))
		protected.GET("/me", authMe)
		protected.POST("/logout", func(c *gin.Context) { authLogout(c, services) })
		protected.POST("/logout-all", func(c *gin.Context) { authLogoutAll(c, services) })
	}
}

// starts oauth flow
// default is redirect but mode=json returns url payload
func discordLogin(c *gin.Context, services *quack.Services) {
	if err := validateDiscordOAuthConfig(services.Config); err != nil {
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "Discord sign-in is unavailable")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(c.DefaultQuery("mode", "redirect")))
	if mode != "json" {
		mode = "redirect"
	}

	stateID, err := idutil.NewULID()
	if err != nil {
		apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "could not start Discord sign-in")
		return
	}

	redirectTo := sanitizeRedirectTarget(c.Query("redirect_to"), services.Config.Auth.PostLoginRedirect)
	stateTTL := time.Duration(services.Config.Auth.StateTTLMinutes) * time.Minute
	statePayload := &model.OAuthState{
		RedirectTo:   redirectTo,
		ResponseMode: mode,
		CreatedAt:    time.Now().UTC(),
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := services.Store.SaveOAuthState(ctx, stateID, statePayload, stateTTL); err != nil {
		log.Error().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("oauth state dependency unavailable")
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "Discord sign-in is temporarily unavailable")
		return
	}

	authURL := buildDiscordAuthURL(services.Config, stateID)
	if mode == "json" {
		c.JSON(http.StatusOK, gin.H{"auth_url": authURL, "state": stateID})
		return
	}

	c.Redirect(http.StatusFound, authURL)
}

// handles oauth callback from discord
func discordCallback(c *gin.Context, services *quack.Services) {
	if err := validateDiscordOAuthConfig(services.Config); err != nil {
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "Discord sign-in is unavailable")
		return
	}

	if oauthErr := c.Query("error"); oauthErr != "" {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeReauthenticate, "Discord authorization was not granted; sign in again")
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	state := strings.TrimSpace(c.Query("state"))
	if code == "" || state == "" {
		apierror.Write(c, http.StatusBadRequest, apierror.CodeValidation, "OAuth code and state are required")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	statePayload, err := services.Store.ConsumeOAuthState(ctx, state)
	if err != nil {
		log.Error().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("oauth state dependency unavailable")
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "Discord sign-in is temporarily unavailable")
		return
	}
	if statePayload == nil {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeReauthenticate, "Discord sign-in expired; sign in again")
		return
	}

	tokenData, err := exchangeDiscordCode(ctx, services.Config, code)
	if err != nil {
		log.Warn().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("Discord OAuth grant rejected")
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeReauthenticate, "Discord authorization is invalid or revoked; sign in again")
		return
	}

	user, err := fetchDiscordUser(ctx, tokenData.AccessToken)
	if err != nil {
		log.Warn().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("Discord OAuth identity request rejected")
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeReauthenticate, "Discord authorization is invalid or revoked; sign in again")
		return
	}

	sessionID, err := idutil.NewULID()
	if err != nil {
		apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "could not create authentication session")
		return
	}
	csrfToken, err := middleware.NewCSRFToken()
	if err != nil {
		apierror.Write(c, http.StatusInternalServerError, apierror.CodeInternal, "could not create authentication session")
		return
	}

	now := time.Now().UTC()
	sessionTTL := time.Duration(services.Config.Auth.SessionTTLHours) * time.Hour
	session := &model.AuthSession{
		ID:               sessionID,
		DiscordUserID:    user.ID,
		Username:         user.Username,
		GlobalName:       user.GlobalName,
		Avatar:           user.Avatar,
		AccessToken:      tokenData.AccessToken,
		RefreshToken:     tokenData.RefreshToken,
		CSRFToken:        csrfToken,
		TokenType:        tokenData.TokenType,
		Scope:            tokenData.Scope,
		TokenExpiresAt:   now.Add(time.Duration(tokenData.ExpiresIn) * time.Second),
		SessionExpiresAt: now.Add(sessionTTL),
		CreatedAt:        now,
		LastSeenAt:       now,
	}

	if err := services.Store.SaveSession(ctx, session, sessionTTL); err != nil {
		log.Error().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("auth session dependency unavailable")
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "authentication service unavailable")
		return
	}

	setAuthCookies(c, services.Config, sessionID, csrfToken, int(sessionTTL.Seconds()))

	if statePayload.ResponseMode == "json" {
		c.JSON(http.StatusOK, gin.H{
			"csrf_token": csrfToken,
			"user": gin.H{
				"id":          session.DiscordUserID,
				"username":    session.Username,
				"global_name": session.GlobalName,
				"avatar":      session.Avatar,
				"avatar_url":  discordAvatarURL(session.DiscordUserID, session.Avatar),
			},
			"expires_at": session.SessionExpiresAt,
		})
		return
	}

	c.Redirect(http.StatusFound, sanitizeRedirectTarget(statePayload.RedirectTo, services.Config.Auth.PostLoginRedirect))
}

// simple whoami endpoint
func authMe(c *gin.Context) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeAuthentication, "authentication required")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"csrf_token": session.CSRFToken,
		"user": gin.H{
			"id":          session.DiscordUserID,
			"username":    session.Username,
			"global_name": session.GlobalName,
			"avatar":      session.Avatar,
			"avatar_url":  discordAvatarURL(session.DiscordUserID, session.Avatar),
		},
		"session": gin.H{
			"expires_at": session.SessionExpiresAt,
			"last_seen":  session.LastSeenAt,
		},
	})
}

// handles logout request
func authLogout(c *gin.Context, services *quack.Services) {
	session := middleware.GetAuthSession(c)
	if session != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()
		if err := services.Store.DeleteSession(ctx, session.ID); err != nil {
			log.Error().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("auth logout dependency unavailable")
			apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "authentication service unavailable")
			return
		}
	}

	clearAuthCookies(c, services.Config)
	c.Status(http.StatusNoContent)
}

// authLogoutAll revokes all indexed sessions for compromise response or account changes.
func authLogoutAll(c *gin.Context, services *quack.Services) {
	session := middleware.GetAuthSession(c)
	if session == nil {
		apierror.Write(c, http.StatusUnauthorized, apierror.CodeAuthentication, "authentication required")
		return
	}
	revoker, ok := services.Store.(userSessionRevoker)
	if !ok {
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "session revocation unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := revoker.RevokeUserSessions(ctx, session.DiscordUserID); err != nil {
		log.Error().Str("request_id", quack.RequestIDFromContext(c.Request.Context())).Msg("auth compromise revocation dependency unavailable")
		apierror.Write(c, http.StatusServiceUnavailable, apierror.CodeDependency, "session revocation unavailable")
		return
	}
	clearAuthCookies(c, services.Config)
	c.Status(http.StatusNoContent)
}

// builds discord oauth authorization url
func buildDiscordAuthURL(cfg config.Config, state string) string {
	v := url.Values{}
	v.Set("client_id", cfg.Discord.AppID)
	v.Set("redirect_uri", cfg.Discord.OAuthRedirectURI)
	v.Set("response_type", "code")
	v.Set("scope", cfg.Discord.OAuthScopes)
	v.Set("state", state)

	return discordAuthorizeURL + "?" + v.Encode()
}

// exchanges discord authorization code for access token
func exchangeDiscordCode(ctx context.Context, cfg config.Config, code string) (*discordTokenResponse, error) {
	body := url.Values{}
	body.Set("client_id", cfg.Discord.AppID)
	body.Set("client_secret", cfg.Discord.ClientSecret)
	body.Set("grant_type", "authorization_code")
	body.Set("code", code)
	body.Set("redirect_uri", cfg.Discord.OAuthRedirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, discordTokenEndpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := discordHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var token discordTokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDiscordBodyBytes)).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	if resp.StatusCode >= 400 || token.AccessToken == "" || token.ExpiresIn <= 0 {
		return nil, fmt.Errorf("discord token exchange rejected")
	}

	return &token, nil
}

// fetches discord user information
func fetchDiscordUser(ctx context.Context, accessToken string) (*discordUserResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discordMeEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create user request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := discordHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user request failed: %w", err)
	}
	defer resp.Body.Close()

	var user discordUserResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxDiscordBodyBytes)).Decode(&user); err != nil {
		return nil, fmt.Errorf("decode user response: %w", err)
	}

	if resp.StatusCode >= 400 || user.ID == "" {
		return nil, fmt.Errorf("discord user fetch rejected")
	}

	return &user, nil
}

// setAuthCookies sets explicit production-safe session and double-submit CSRF cookies.
func setAuthCookies(c *gin.Context, cfg config.Config, sessionID, csrfToken string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		cfg.Auth.SessionCookieName,
		sessionID,
		maxAge,
		"/",
		"",
		cfg.Auth.CookieSecure,
		true,
	)
	c.SetCookie(cfg.Auth.CSRFCookieName, csrfToken, maxAge, "/", "", cfg.Auth.CookieSecure, false)
}

// clearAuthCookies clears both browser authentication credentials.
func clearAuthCookies(c *gin.Context, cfg config.Config) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		cfg.Auth.SessionCookieName,
		"",
		-1,
		"/",
		"",
		cfg.Auth.CookieSecure,
		true,
	)
	c.SetCookie(cfg.Auth.CSRFCookieName, "", -1, "/", "", cfg.Auth.CookieSecure, false)
}

// builds discord avatar url
func discordAvatarURL(userID, avatarHash string) string {
	if userID == "" || avatarHash == "" {
		return ""
	}

	ext := "png"
	if strings.HasPrefix(avatarHash, "a_") {
		ext = "gif"
	}

	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.%s", userID, avatarHash, ext)
}

// sanitizes redirect target
func sanitizeRedirectTarget(target, fallback string) string {
	target = strings.TrimSpace(target)
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		fallback = "/"
	}

	if target == "" {
		return fallback
	}

	if strings.HasPrefix(target, "/") {
		return target
	}

	targetURL, err := url.Parse(target)
	if err != nil || targetURL.Scheme == "" || targetURL.Host == "" {
		return fallback
	}

	fallbackURL, err := url.Parse(fallback)
	if err != nil {
		return "/"
	}

	if fallbackURL.Host == "" {
		return fallback
	}

	if strings.EqualFold(targetURL.Host, fallbackURL.Host) {
		return target
	}

	return fallback
}

// validates discord oauth configuration
func validateDiscordOAuthConfig(cfg config.Config) error {
	if strings.TrimSpace(cfg.Discord.AppID) == "" {
		return fmt.Errorf("discord oauth is not configured missing DISCORD_APP_ID")
	}
	if strings.TrimSpace(cfg.Discord.ClientSecret) == "" {
		return fmt.Errorf("discord oauth is not configured missing DISCORD_CLIENT_SECRET")
	}
	if strings.TrimSpace(cfg.Discord.OAuthRedirectURI) == "" {
		return fmt.Errorf("discord oauth is not configured missing DISCORD_OAUTH_REDIRECT_URI")
	}
	return nil
}
