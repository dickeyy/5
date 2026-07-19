package platform

import "github.com/redis/go-redis/v9"

// RedisProvider is the narrow adapter contract used by the integration checkpoint to construct HTTP safety primitives.
type RedisProvider interface {
	Redis() *redis.Client
}

// Primitives groups the reusable rate-limit and idempotency contracts exposed to feature registrars.
type Primitives struct {
	RateLimits  *RateLimiter
	Idempotency *IdempotencyStore
}

// FromRepository constructs primitives from an adapter that explicitly exposes its owned Redis client.
func FromRepository(repository any) Primitives {
	provider, _ := repository.(RedisProvider)
	if provider == nil {
		return Primitives{RateLimits: NewRateLimiter(nil, ""), Idempotency: NewIdempotencyStore(nil, "")}
	}
	client := provider.Redis()
	return Primitives{RateLimits: NewRateLimiter(client, ""), Idempotency: NewIdempotencyStore(client, "")}
}
