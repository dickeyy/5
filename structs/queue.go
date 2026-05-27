package structs

import (
	"context"
	"time"

	r "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type DataStore interface {
	DB() *gorm.DB
	Redis() *r.Client
}

type QueueEvent struct {
	ID            string
	Type          string
	CreatedAt     time.Time
	RequestID     string
	CorrelationID string
	Data          any
	Handler       func(ctx context.Context, s DataStore, data any) error
}
