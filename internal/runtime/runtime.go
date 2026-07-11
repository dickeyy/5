package runtime

import (
	"context"
	"fmt"

	"github.com/quackdiscord/bot/internal/config"
	"github.com/quackdiscord/bot/internal/discordbot"
	"github.com/quackdiscord/bot/internal/discordbot/commands"
	"github.com/quackdiscord/bot/internal/httpapi"
	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/store"
	"github.com/quackdiscord/bot/internal/workqueue"
	"github.com/rs/zerolog/log"
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
	if err := bot.Open(); err != nil {
		return fmt.Errorf("connect Discord bot: %w", err)
	}
	defer bot.Close()

	queue := workqueue.New(cfg.EventQueue.Size, cfg.EventQueue.Workers)
	services := quack.NewWithConfigDependencies(cfg, repositories, bot, bot, queue)
	queue.Start(ctx, services.Actions.ProcessCaseActions, repositories)
	defer queue.Stop()

	if err := commands.Register(bot.Session, services); err != nil {
		log.Error().Err(err).Msg("Failed to register Discord commands")
	}
	return httpapi.Run(ctx, cfg, services, bot)
}
