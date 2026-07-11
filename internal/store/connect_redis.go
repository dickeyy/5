package store

import (
	"context"
	"fmt"

	r "github.com/redis/go-redis/v9"
)

// OpenRedis opens and verifies redis so startup fails before serving traffic when the dependency is unavailable.
func OpenRedis(rawURL string) (*r.Client, error) {
	options, err := r.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse Redis URL: %w", err)
	}
	client := r.NewClient(options)
	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("ping Redis: %w", err)
	}
	return client, nil
}
