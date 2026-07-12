package moduleintegration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bwmarrin/discordgo"
	"github.com/gin-gonic/gin"
	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/httpapi/middleware"
	httpplatform "github.com/quackdiscord/bot/internal/httpapi/platform"
	"github.com/quackdiscord/bot/internal/modules"
	"github.com/quackdiscord/bot/internal/modules/generallogging"
	"github.com/quackdiscord/bot/internal/modules/tickets"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
	"github.com/quackdiscord/bot/internal/store"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestModuleAuditAdapterWritesImmutableCoreEntry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:module-audit?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repository := store.New(db, nil)
	if err := repository.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	guild := model.Guild{ULIDModel: model.ULIDModel{ID: "01J60000000000000000000001"}, DiscordGuildID: "discord-guild", Name: "Guild", IsActive: true}
	if err := db.Create(&guild).Error; err != nil {
		t.Fatalf("create guild: %v", err)
	}
	auditor := moduleAuditor{repository: repository}
	if err := auditor.RecordModuleAudit(context.Background(), modules.AuditEvent{
		GuildID: guild.ID, ActorDiscordUserID: "actor", Action: "ticket.open",
		ResourceType: "ticket", ResourceID: "ticket-1", Result: "success", MetadataJSON: "{}",
	}); err != nil {
		t.Fatalf("record module audit: %v", err)
	}
	entries, err := repository.ListAuditLogEntries(context.Background(), guild.ID)
	if err != nil || len(entries) != 1 {
		t.Fatalf("read module audit: entries=%+v err=%v", entries, err)
	}
	if entries[0].Source != model.AuditSourceSystem || entries[0].Action != "ticket.open" {
		t.Fatalf("unexpected module audit: %+v", entries[0])
	}
}

func TestPrivateChannelACLRequiresEveryoneDenialAndStaffVisibility(t *testing.T) {
	channel := &discordgo.Channel{GuildID: "guild", PermissionOverwrites: ticketPermissionOverwrites("guild", "owner", "bot", []string{"staff"})}
	if err := validateTicketACL(channel, "guild", "owner", "bot", []string{"staff"}); err != nil {
		t.Fatalf("valid private ACL rejected: %v", err)
	}
	channel.PermissionOverwrites = channel.PermissionOverwrites[1:]
	if err := validateTicketACL(channel, "guild", "owner", "bot", []string{"staff"}); err == nil {
		t.Fatal("public ticket ACL was accepted")
	}
}

func TestOptionalModuleHTTPRegistrarsMountCompleteSurface(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:module-routes?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := modules.RegistryMigration().Apply(db); err != nil {
		t.Fatalf("migrate registry: %v", err)
	}
	if err := tickets.Migration().Apply(db); err != nil {
		t.Fatalf("migrate tickets: %v", err)
	}
	registry, err := modules.NewRegistry(modules.NewSQLSettingsStore(db), tickets.Descriptor(), generallogging.Descriptor())
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	runtime := &Runtime{
		Tickets: tickets.NewService(registry, tickets.NewStore(db), nil),
		Logging: generallogging.NewService(registry, nil, nil, nil),
	}
	services := quack.NewWithConfigDependencies(config.Default(), store.New(db, nil), nil, nil, nil)
	engine := gin.New()
	if err := runtime.RegisterHTTP(engine.Group("/guilds"), services, httpplatform.FromRepository(services.Store)); err != nil {
		t.Fatalf("register module routes: %v", err)
	}
	want := map[string]bool{
		"GET /guilds/:discordGuildID/modules/tickets/status":               false,
		"GET /guilds/:discordGuildID/modules/tickets/:ticketID/transcript": false,
		"PUT /guilds/:discordGuildID/modules/general-logging/settings":     false,
	}
	for _, route := range engine.Routes() {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; ok {
			want[key] = true
		}
	}
	for route, present := range want {
		if !present {
			t.Fatalf("missing optional module route %s", route)
		}
	}
}

func TestHTTPActorMappingIncludesManagersAndAdministrators(t *testing.T) {
	for _, permissions := range []map[model.PermissionAction]bool{
		{model.PermissionActionGuildSettingsWrite: true},
		{model.PermissionActionGuildSettingsWrite: true, model.PermissionActionTicketResolve: true},
	} {
		ctx, _ := gin.CreateTestContext(nil)
		ctx.Set(middleware.ContextGuildKey, &quack.GuildStaffContext{
			Guild:              &model.Guild{ULIDModel: model.ULIDModel{ID: "guild"}},
			ActorDiscordUserID: "actor", Permissions: permissions,
		})
		actor, err := resolveTicketActor(ctx)
		if err != nil || !actor.CanManage {
			t.Fatalf("Manage Guild actor lost module authority: actor=%+v err=%v", actor, err)
		}
		loggingActor, err := resolveLoggingActor(ctx)
		if err != nil || !loggingActor.CanManage {
			t.Fatalf("Manage Guild actor lost logging authority: actor=%+v err=%v", loggingActor, err)
		}
	}
}

func TestModuleIdempotencyScopeIncludesOperation(t *testing.T) {
	engine := gin.New()
	seen := make(chan string, 2)
	engine.POST("/guilds/:discordGuildID/modules/tickets/:ticketID/reopen", func(c *gin.Context) {
		c.Set(middleware.ContextGuildKey, &quack.GuildStaffContext{Guild: &model.Guild{ULIDModel: model.ULIDModel{ID: "guild"}}, ActorDiscordUserID: "actor"})
		seen <- moduleWriteSubject(c)
	})
	engine.PUT("/guilds/:discordGuildID/modules/tickets/settings", func(c *gin.Context) {
		c.Set(middleware.ContextGuildKey, &quack.GuildStaffContext{Guild: &model.Guild{ULIDModel: model.ULIDModel{ID: "guild"}}, ActorDiscordUserID: "actor"})
		seen <- moduleWriteSubject(c)
	})
	for method, path := range map[string]string{
		http.MethodPost: "/guilds/discord/modules/tickets/ticket/reopen",
		http.MethodPut:  "/guilds/discord/modules/tickets/settings",
	} {
		engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, path, nil))
	}
	first, second := <-seen, <-seen
	if first == second {
		t.Fatalf("distinct module writes shared idempotency scope %q", first)
	}
}

func TestRuntimeWorkerShutdownIsIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:module-runtime?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repository := store.New(db, nil)
	if err := repository.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	session, err := discordgo.New("Bot integration-test")
	if err != nil {
		t.Fatalf("new Discord session: %v", err)
	}
	runtime, err := New(context.Background(), repository, session)
	if err != nil {
		t.Fatalf("new module runtime: %v", err)
	}
	runtime.Close()
	runtime.Close()
}

func TestModuleIdempotencyStoresNormalizedErrors(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	engine := gin.New()
	store := httpplatform.NewIdempotencyStore(client, "qi1:test:")
	engine.Use(store.Protect("module", time.Hour, func(*gin.Context) string { return "actor:guild:operation" }))
	engine.Use(middleware.ErrorEnvelope)
	engine.POST("/write", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token=must-not-persist"})
	})
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/write", nil)
		request.Header.Set("Idempotency-Key", "same-write")
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), "must-not-persist") {
			t.Fatalf("unsafe idempotent error response: status=%d body=%s", response.Code, response.Body.String())
		}
		if attempt == 1 && response.Header().Get("Idempotency-Replayed") != "true" {
			t.Fatal("expected normalized error replay")
		}
	}
}

func TestBulkLoggingSubmissionIsSafeDuringShutdown(t *testing.T) {
	runtime := &Runtime{bulk: make(chan bulkDeleteEvent, 128)}
	var workers sync.WaitGroup
	for i := 0; i < 64; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			runtime.submitBulkDelete(bulkDeleteEvent{guildID: "guild", messageIDs: []string{"message"}})
		}()
	}
	runtime.Close()
	workers.Wait()
	runtime.Close()
}
