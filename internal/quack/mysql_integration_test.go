package quack_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
	"github.com/quackdiscord/bot/internal/store"
)

func TestMySQLConcurrentCaseCreationSelectsDistinctEscalation(t *testing.T) {
	dsn := os.Getenv("QUACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("QUACK_TEST_MYSQL_DSN is not configured")
	}
	db, err := store.OpenMySQL(dsn)
	if err != nil {
		t.Fatal(err)
	}
	repositories := store.New(db, nil)
	if err := repositories.Migrate(); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	suffix := fmt.Sprint(time.Now().UTC().UnixNano())
	guild, err := repositories.UpsertGuild(ctx, model.UpsertGuildParams{
		DiscordGuildID: "integration-" + suffix, Name: "Integration", OwnerDiscordUserID: "owner",
	})
	if err != nil {
		t.Fatal(err)
	}
	staff := &model.StaffMember{GuildID: guild.ID, DiscordUserID: "moderator"}
	guildContext := &quack.GuildStaffContext{
		Guild: guild, Staff: staff, IsAdmin: true, IsModerator: true,
		Permissions: map[model.PermissionAction]bool{
			model.PermissionActionCaseTemplateWrite: true,
			model.PermissionActionCaseCreate:        true,
		},
	}
	services := quack.NewWithConfigDependencies(config.Default(), repositories, nil, nil, nil)
	template, err := services.Templates.Create(ctx, guildContext, quack.TemplateInput{
		Slug: "concurrent-" + suffix, Name: "Concurrent", Description: "Integration", ReasonTemplate: "Reason",
		Levels: []quack.TemplateLevelInput{
			{Name: "Default", Position: 1, IsDefault: true},
			{Name: "Second", Position: 2, TriggerCaseCount: 2},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan *quack.CaseResponse, 2)
	errors := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(2)
	for range 2 {
		go func() {
			start.Done()
			start.Wait()
			created, createErr := services.Cases.Create(ctx, guildContext, quack.CaseInput{
				TemplateID: template.ID, TargetDiscordUserID: "target",
			})
			if createErr != nil {
				errors <- createErr
				return
			}
			results <- created
		}()
	}

	created := make([]*quack.CaseResponse, 0, 2)
	for range 2 {
		select {
		case err := <-errors:
			t.Fatal(err)
		case result := <-results:
			created = append(created, result)
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for concurrent case creation")
		}
	}
	if created[0].CaseNumber == created[1].CaseNumber {
		t.Fatalf("duplicate case number %d", created[0].CaseNumber)
	}
	levels := map[string]bool{}
	for _, result := range created {
		levels[result.SelectedLevel.Name] = true
	}
	if !levels["Default"] || !levels["Second"] {
		t.Fatalf("expected default and second escalation levels, got %+v", levels)
	}
}
