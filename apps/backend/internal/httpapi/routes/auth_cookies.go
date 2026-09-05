package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
)

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

// oauthStateCookieName uses a host-bound production cookie to prevent subdomain cookie injection.
func oauthStateCookieName(cfg config.Config) string {
	if cfg.Auth.CookieSecure {
		return "__Host-quack_oauth_state"
	}
	return "quack_oauth_state"
}

// setOAuthStateCookie binds the single-use OAuth challenge to its initiating browser.
func setOAuthStateCookie(c *gin.Context, cfg config.Config, state string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(oauthStateCookieName(cfg), state, maxAge, "/", "", cfg.Auth.CookieSecure, true)
}
