package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/quackdiscord/bot/internal/config"
)

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
