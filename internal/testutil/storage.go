package testutil

import (
	"testing"

	"github.com/quackdiscord/bot/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewSQLiteStore(t testing.TB) *storage.Store {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test database: %v", err)
	}

	return storage.New(db, nil)
}
