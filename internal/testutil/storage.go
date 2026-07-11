package testutil

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/quackdiscord/bot/internal/store"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// NewSQLiteStore constructs sqlite store with required dependencies explicit so callers control lifecycle and substitution.
func NewSQLiteStore(t testing.TB) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}

	return store.New(db, nil)
}

// NewSQLiteRedisStore constructs sqlite redis store with required dependencies explicit so callers control lifecycle and substitution.
func NewSQLiteRedisStore(t testing.TB) *store.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}

	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})

	return store.New(db, redisClient)
}
