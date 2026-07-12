package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	"github.com/redis/go-redis/v9"
)

func TestEndpointPolicyRequiresAndReplaysIdempotentWrites(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cfg := config.Default()
	primitives := Primitives{RateLimits: NewRateLimiter(client, "test:route-rate:"), Idempotency: NewIdempotencyStore(client, "test:route-idempotency:")}
	router := gin.New()
	router.Use(middleware.RequestContext, middleware.ErrorEnvelope, EndpointPolicy(primitives, cfg))
	calls := 0
	router.PUT("/guilds/:discordGuildID/modules/example/settings", func(c *gin.Context) { calls++; c.JSON(http.StatusOK, gin.H{"saved": true}) })

	request := httptest.NewRequest(http.MethodPut, "/guilds/guild-1/modules/example/settings", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || calls != 0 {
		t.Fatalf("expected missing idempotency key to stop write: status=%d calls=%d body=%s", response.Code, calls, response.Body.String())
	}

	for attempt := 0; attempt < 2; attempt++ {
		request = httptest.NewRequest(http.MethodPut, "/guilds/guild-1/modules/example/settings", nil)
		request.Header.Set("Authorization", "Bearer session")
		request.Header.Set("Idempotency-Key", "settings-update")
		response = httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("write attempt %d: status=%d body=%s", attempt, response.Code, response.Body.String())
		}
		if attempt == 1 && response.Header().Get("Idempotency-Replayed") != "true" {
			t.Fatal("expected durable replay marker")
		}
	}
	if calls != 1 {
		t.Fatalf("expected one feature execution, got %d", calls)
	}
}

func TestEndpointPolicyFailsClosedWhenRedisUnavailable(t *testing.T) {
	cfg := config.Default()
	router := gin.New()
	router.Use(middleware.RequestContext, middleware.ErrorEnvelope, EndpointPolicy(Primitives{RateLimits: NewRateLimiter(nil, ""), Idempotency: NewIdempotencyStore(nil, "")}, cfg))
	router.GET("/members/me/cases/:caseID", func(c *gin.Context) { c.Status(http.StatusOK) })
	request := httptest.NewRequest(http.MethodGet, "/members/me/cases/case-1", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected unavailable Redis to fail closed, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestEndpointRatePolicyMatrix(t *testing.T) {
	cfg := config.Default()
	tests := []struct {
		method, path, class string
	}{
		{http.MethodGet, "/members/me/cases/case-1", "member-read"},
		{http.MethodPut, "/guilds/guild-1/modules/tickets/settings", "authenticated-write"},
		{http.MethodPost, "/guilds/guild-1/cases", "case-create"},
		{http.MethodPost, "/guilds/guild-1/action-failures/action-1/retry", "action-recovery"},
		{http.MethodPost, "/guilds/guild-1/cases/case-1/evidence", "evidence"},
	}
	for _, test := range tests {
		class, limit, applicable := endpointRatePolicy(test.method, test.path, cfg)
		if !applicable || class != test.class || limit.Maximum <= 0 || limit.Window <= 0 {
			t.Errorf("%s %s: got class=%q limit=%+v applicable=%v", test.method, test.path, class, limit, applicable)
		}
	}
	if _, _, applicable := endpointRatePolicy(http.MethodGet, "/livez", cfg); applicable {
		t.Fatal("health probes must not depend on actor rate-limit policy")
	}
}
