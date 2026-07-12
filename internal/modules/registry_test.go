package modules_test

import (
	"context"
	"testing"

	"github.com/quackdiscord/bot/internal/modules"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegistryKeepsGuildsAndModulesIndependent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:module-registry?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	migration := modules.RegistryMigration()
	if migration.Version != 100 {
		t.Fatalf("migration version = %d", migration.Version)
	}
	if err := migration.Apply(db); err != nil {
		t.Fatal(err)
	}
	registry, err := modules.NewRegistry(modules.NewSQLSettingsStore(db),
		modules.Descriptor{ID: modules.Tickets, DisplayName: "Tickets"},
		modules.Descriptor{ID: modules.GeneralLogging, DisplayName: "General logging"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := registry.SetConfiguration(ctx, modules.Configuration{GuildID: "guild-a", ModuleID: modules.Tickets, Enabled: true, ConfigJSON: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.SetConfiguration(ctx, modules.Configuration{GuildID: "guild-a", ModuleID: modules.GeneralLogging, Enabled: false, ConfigJSON: `{}`}); err != nil {
		t.Fatal(err)
	}
	if got, err := registry.Configuration(ctx, "guild-a", modules.Tickets); err != nil || got == nil || !got.Enabled {
		t.Fatalf("tickets: %+v, %v", got, err)
	}
	if got, err := registry.Configuration(ctx, "guild-a", modules.GeneralLogging); err != nil || got == nil || got.Enabled {
		t.Fatalf("logging: %+v, %v", got, err)
	}
	if got, err := registry.Configuration(ctx, "guild-b", modules.Tickets); err != nil || got != nil {
		t.Fatalf("guild leak: %+v, %v", got, err)
	}
}
