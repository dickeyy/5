package runtime

import (
	"context"
	"fmt"

	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/discordbot"
	"github.com/quackdiscord/bot/internal/discordbot/commands"
	"github.com/quackdiscord/bot/internal/httpapi"
	"github.com/quackdiscord/bot/internal/moduleintegration"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/store"
	"github.com/quackdiscord/bot/internal/workqueue"
)

// Run assembles every adapter around the application core, starts the process, and shuts dependencies down in reverse order.
func Run(ctx context.Context) error {
	cfg := config.Load()
	db, err := store.OpenMySQL(cfg.Storage.DBDSN)
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	redis, err := store.OpenRedis(cfg.Storage.RedisURL)
	if err != nil {
		return err
	}
	defer redis.Close()

	repositories := store.New(db, redis)
	if err := repositories.Migrate(); err != nil {
		return fmt.Errorf("migrate storage: %w", err)
	}

	bot, err := discordbot.New(cfg.Discord.Token)
	if err != nil {
		return fmt.Errorf("create Discord bot: %w", err)
	}
	queue := workqueue.New(cfg.EventQueue.Size, cfg.EventQueue.Workers)
	services := quack.NewWithConfigDependencies(cfg, repositories, bot, bot, queue)
	moduleRuntime, err := moduleintegration.New(ctx, repositories, bot.Session, services, bot)
	if err != nil {
		return fmt.Errorf("compose optional modules: %w", err)
	}
	defer moduleRuntime.Close()
	intents, err := moduleRuntime.RequiredGatewayIntents(ctx)
	if err != nil {
		return fmt.Errorf("derive optional module gateway intents: %w", err)
	}
	bot.Session.Identify.Intents = intents
	if err := discordbot.RegisterGuildLifecycle(bot.Session, services); err != nil {
		return fmt.Errorf("register Discord guild lifecycle: %w", err)
	}
	if err := moduleRuntime.RegisterGatewayHandlers(bot.Session); err != nil {
		return fmt.Errorf("register optional module gateway handlers: %w", err)
	}
	if err := commands.Register(bot.Session, services, moduleRuntime.RegisterComponents); err != nil {
		return fmt.Errorf("register Discord commands and components: %w", err)
	}
	if err := bot.Open(); err != nil {
		return fmt.Errorf("connect Discord bot: %w", err)
	}
	defer bot.Close()

	queue.Start(ctx, services.Actions.ProcessCaseActions, repositories)
	defer queue.Stop()

	return httpapi.Run(ctx, cfg, services, moduleRuntime, bot)
}
