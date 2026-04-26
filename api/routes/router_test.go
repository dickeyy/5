package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/app"
	"github.com/quackdiscord/bot/internal/testutil"
	"github.com/quackdiscord/bot/structs"
)

func TestSetupRoutesStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	SetupRoutes(router, app.New(nil))

	request := httptest.NewRequest(http.MethodGet, "/status", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body map[string]map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode status response: %v", err)
	}

	if body["discord"]["connected"] != false {
		t.Fatalf("expected discord to be disconnected in route smoke test")
	}
	if body["redis"]["connected"] != false {
		t.Fatalf("expected redis to be disconnected in route smoke test")
	}
	if body["database"]["connected"] != false {
		t.Fatalf("expected database to be disconnected in route smoke test")
	}
}

func TestGuildMeRouteAuthenticated(t *testing.T) {
	testutil.SetTestConfig(t)
	gin.SetMode(gin.TestMode)

	store := testutil.NewSQLiteRedisStore(t)
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	services := app.NewWithDiscordClient(store, routeFakeDiscordClient{
		userGuilds: []app.DiscordUserGuild{{
			ID:          "guild-1",
			Permissions: uint64(discordgo.PermissionModerateMembers),
		}},
		botGuild: &app.DiscordBotGuild{ID: "guild-1", Name: "Guild", Icon: "icon", OwnerID: "owner-1"},
	})

	session := routeTestSession("user-1")
	if err := store.SaveSession(context.Background(), session, time.Hour); err != nil {
		t.Fatalf("save session: %v", err)
	}

	router := gin.New()
	SetupRoutes(router, services)

	request := httptest.NewRequest(http.MethodGet, "/guilds/guild-1/me", nil)
	request.Header.Set("Authorization", "Bearer "+session.ID)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, response.Code, response.Body.String())
	}

	var body struct {
		Guild struct {
			ID               string `json:"id"`
			DiscordGuildID   string `json:"discord_guild_id"`
			Name             string `json:"name"`
			OwnerDiscordUser string `json:"owner_discord_user_id"`
		} `json:"guild"`
		Staff struct {
			ID             string `json:"id"`
			DiscordUserID  string `json:"discord_user_id"`
			PermissionBits string `json:"permission_bits"`
		} `json:"staff"`
		Permissions map[string]bool `json:"permissions"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode guild me response: %v", err)
	}

	if body.Guild.ID == "" || body.Guild.DiscordGuildID != "guild-1" || body.Guild.Name != "Guild" {
		t.Fatalf("unexpected guild payload: %+v", body.Guild)
	}
	if body.Staff.ID == "" || body.Staff.DiscordUserID != "user-1" {
		t.Fatalf("unexpected staff payload: %+v", body.Staff)
	}
	if body.Permissions[string(structs.PermissionActionCaseCreate)] != true {
		t.Fatalf("expected case.create permission")
	}
	if body.Permissions[string(structs.PermissionActionCaseTemplateWrite)] != false {
		t.Fatalf("did not expect case_template.write permission")
	}
}

func TestGuildMeRouteUnauthenticated(t *testing.T) {
	testutil.SetTestConfig(t)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	SetupRoutes(router, app.New(nil))

	request := httptest.NewRequest(http.MethodGet, "/guilds/guild-1/me", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
	}
}

func TestTemplateRoutesReadAndWritePermissions(t *testing.T) {
	router, sessionID := newTemplateRouteHarness(t, uint64(discordgo.PermissionModerateMembers))

	listRequest := httptest.NewRequest(http.MethodGet, "/guilds/guild-1/templates", nil)
	listRequest.Header.Set("Authorization", "Bearer "+sessionID)
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected read status %d, got %d body=%s", http.StatusOK, listResponse.Code, listResponse.Body.String())
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/guilds/guild-1/templates", bytes.NewBufferString(templateRoutePayload("spam")))
	createRequest.Header.Set("Authorization", "Bearer "+sessionID)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusForbidden {
		t.Fatalf("expected write status %d, got %d body=%s", http.StatusForbidden, createResponse.Code, createResponse.Body.String())
	}

	patchRequest := httptest.NewRequest(http.MethodPatch, "/guilds/guild-1/templates/template-1", bytes.NewBufferString(templateRoutePayload("spam")))
	patchRequest.Header.Set("Authorization", "Bearer "+sessionID)
	patchRequest.Header.Set("Content-Type", "application/json")
	patchResponse := httptest.NewRecorder()
	router.ServeHTTP(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusForbidden {
		t.Fatalf("expected patch status %d, got %d body=%s", http.StatusForbidden, patchResponse.Code, patchResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/guilds/guild-1/templates/template-1", nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+sessionID)
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusForbidden {
		t.Fatalf("expected delete status %d, got %d body=%s", http.StatusForbidden, deleteResponse.Code, deleteResponse.Body.String())
	}
}

func TestTemplateRoutesCreateUpdateDelete(t *testing.T) {
	router, sessionID := newTemplateRouteHarness(t, uint64(discordgo.PermissionManageGuild))

	createRequest := httptest.NewRequest(http.MethodPost, "/guilds/guild-1/templates", bytes.NewBufferString(templateRoutePayload("spam")))
	createRequest.Header.Set("Authorization", "Bearer "+sessionID)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("expected create status %d, got %d body=%s", http.StatusCreated, createResponse.Code, createResponse.Body.String())
	}

	var createBody struct {
		Template struct {
			ID      string `json:"id"`
			Slug    string `json:"slug"`
			Version uint   `json:"version"`
		} `json:"template"`
	}
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createBody); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createBody.Template.ID == "" || createBody.Template.Slug != "spam" {
		t.Fatalf("unexpected create body: %+v", createBody.Template)
	}

	updateRequest := httptest.NewRequest(http.MethodPatch, "/guilds/guild-1/templates/"+createBody.Template.ID, bytes.NewBufferString(templateRoutePayload("spam-updated")))
	updateRequest.Header.Set("Authorization", "Bearer "+sessionID)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("expected update status %d, got %d body=%s", http.StatusOK, updateResponse.Code, updateResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/guilds/guild-1/templates/"+createBody.Template.ID, nil)
	deleteRequest.Header.Set("Authorization", "Bearer "+sessionID)
	deleteResponse := httptest.NewRecorder()
	router.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("expected delete status %d, got %d body=%s", http.StatusOK, deleteResponse.Code, deleteResponse.Body.String())
	}
}

type routeFakeDiscordClient struct {
	userGuilds []app.DiscordUserGuild
	botGuild   *app.DiscordBotGuild
}

func (f routeFakeDiscordClient) UserGuilds(ctx context.Context, accessToken string) ([]app.DiscordUserGuild, error) {
	return f.userGuilds, nil
}

func (f routeFakeDiscordClient) BotGuild(ctx context.Context, discordGuildID string) (*app.DiscordBotGuild, error) {
	return f.botGuild, nil
}

func routeTestSession(discordUserID string) *structs.AuthSession {
	now := time.Now().UTC()
	return &structs.AuthSession{
		ID:               "session-1",
		DiscordUserID:    discordUserID,
		Username:         "user",
		GlobalName:       "User",
		AccessToken:      "token",
		SessionExpiresAt: now.Add(time.Hour),
		CreatedAt:        now,
		LastSeenAt:       now,
	}
}

func newTemplateRouteHarness(t *testing.T, permissionBits uint64) (*gin.Engine, string) {
	t.Helper()

	testutil.SetTestConfig(t)
	gin.SetMode(gin.TestMode)

	store := testutil.NewSQLiteRedisStore(t)
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate schema: %v", err)
	}

	services := app.NewWithDiscordClient(store, routeFakeDiscordClient{
		userGuilds: []app.DiscordUserGuild{{
			ID:          "guild-1",
			Permissions: permissionBits,
		}},
		botGuild: &app.DiscordBotGuild{ID: "guild-1", Name: "Guild", OwnerID: "owner-1"},
	})

	session := routeTestSession("user-1")
	if err := store.SaveSession(context.Background(), session, time.Hour); err != nil {
		t.Fatalf("save session: %v", err)
	}

	router := gin.New()
	SetupRoutes(router, services)

	return router, session.ID
}

func templateRoutePayload(slug string) string {
	return `{
		"slug": "` + slug + `",
		"name": "Spam",
		"description": "Spam template",
		"reason_template": "No spam",
		"default_severity": "medium",
		"default_weight": 1,
		"actions": [
			{
				"action_type": "record_warning",
				"config": {},
				"idempotency_scope": "case",
				"enabled": true
			}
		],
		"escalation_rules": []
	}`
}
