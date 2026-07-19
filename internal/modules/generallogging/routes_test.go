package generallogging_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	logmodule "github.com/quackdiscord/bot/internal/modules/generallogging"
)

func TestRouteRegistrarStatusAndAuthorization(t *testing.T) {
	_, service, _, _ := setup(t)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	logmodule.RegisterRoutes(engine.Group("/guilds/:guildID/modules"), service, func(c *gin.Context) (logmodule.Actor, error) {
		return logmodule.Actor{GuildID: c.Param("guildID"), DiscordUserID: "admin", CanManage: true}, nil
	})
	request := httptest.NewRequest(http.MethodGet, "/guilds/guild-a/modules/general-logging/status", nil)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
