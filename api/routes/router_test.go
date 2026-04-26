package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/app"
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
