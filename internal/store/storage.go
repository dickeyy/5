package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/quackdiscord/bot/internal/quack"
	"github.com/quackdiscord/bot/internal/quack/model"
	r "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Store owns the MySQL and Redis clients and implements the core's narrow persistence ports. Raw clients remain adapter-local so transport and domain packages cannot bypass repository rules.
type Store struct {
	db    *gorm.DB
	redis *r.Client
}

// WithGuildCaseLock runs case creation in one transaction while locking the guild row to serialize numbering and escalation selection.
func (s *Store) WithGuildCaseLock(ctx context.Context, guildID string, fn func(quack.Repository) error) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}
	if guildID == "" {
		return errors.New("guild id is required")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var guild model.Guild
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", guildID).Limit(1).Find(&guild)
		if result.Error != nil {
			return fmt.Errorf("lock guild for case creation: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return errors.New("guild not found")
		}
		return fn(New(tx, s.redis))
	})
}

// PingDatabase verifies database connectivity for health reporting.
func (s *Store) PingDatabase(ctx context.Context) error {
	if s == nil || s.db == nil {
		return errors.New("database not connected")
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// PingRedis verifies redis connectivity for health reporting.
func (s *Store) PingRedis(ctx context.Context) error {
	if s == nil || s.redis == nil {
		return errors.New("redis not connected")
	}
	return s.redis.Ping(ctx).Err()
}

// HashGet computes a stable digest for hash get so unchanged Discord commands can skip synchronization.
func (s *Store) HashGet(ctx context.Context, key, field string) ([]byte, error) {
	if s == nil || s.redis == nil {
		return nil, errors.New("redis not connected")
	}
	return s.redis.HGet(ctx, key, field).Bytes()
}

// HashSet computes a stable digest for hash set so unchanged Discord commands can skip synchronization.
func (s *Store) HashSet(ctx context.Context, key, field string, value []byte) error {
	if s == nil || s.redis == nil {
		return errors.New("redis not connected")
	}
	return s.redis.HSet(ctx, key, field, value).Err()
}

// Redis returns the owned Redis client only for adapter-local features such as command caching.
func (s *Store) Redis() *r.Client {
	return s.redis
}

// DB returns the owned GORM handle only for adapter-local integration and migration code.
func (s *Store) DB() *gorm.DB {
	return s.db
}

// New constructs new with required dependencies explicit so callers control lifecycle and substitution.
func New(db *gorm.DB, redis *r.Client) *Store {
	return &Store{db: db, redis: redis}
}
