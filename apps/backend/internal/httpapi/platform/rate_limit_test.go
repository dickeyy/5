package platform

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newRedisHarness creates an isolated in-memory Redis protocol server.
func newRedisHarness(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return server, client
}

func TestRateLimiterAllowanceExpiryAndUnavailable(t *testing.T) {
	server, client := newRedisHarness(t)
	limiter := NewRateLimiter(client, "test:rate:")
	limit := RateLimit{Maximum: 2, Window: time.Minute}

	first, err := limiter.Allow(context.Background(), "actor-secret", limit)
	if err != nil || !first.Allowed || first.Remaining != 1 {
		t.Fatalf("unexpected first decision: %+v err=%v", first, err)
	}
	second, err := limiter.Allow(context.Background(), "actor-secret", limit)
	if err != nil || !second.Allowed || second.Remaining != 0 {
		t.Fatalf("unexpected second decision: %+v err=%v", second, err)
	}
	denied, err := limiter.Allow(context.Background(), "actor-secret", limit)
	if err != nil || denied.Allowed || denied.RetryAfter <= 0 {
		t.Fatalf("unexpected denied decision: %+v err=%v", denied, err)
	}
	for _, key := range server.Keys() {
		if key == "actor-secret" || len(key) < len("test:rate:")+64 {
			t.Fatalf("expected opaque hashed Redis key, got %q", key)
		}
	}
	server.FastForward(time.Minute)
	afterExpiry, err := limiter.Allow(context.Background(), "actor-secret", limit)
	if err != nil || !afterExpiry.Allowed || afterExpiry.Remaining != 1 {
		t.Fatalf("unexpected post-expiry decision: %+v err=%v", afterExpiry, err)
	}

	if decision, err := NewRateLimiter(nil, "").Allow(context.Background(), "actor", limit); !errors.Is(err, ErrUnavailable) || decision.Allowed {
		t.Fatalf("expected deterministic fail-closed unavailable decision, got %+v err=%v", decision, err)
	}
}

func TestRateLimiterConcurrentBoundary(t *testing.T) {
	_, client := newRedisHarness(t)
	limiter := NewRateLimiter(client, "test:race:")
	var allowed atomic.Int64
	var wg sync.WaitGroup
	for range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := limiter.Allow(context.Background(), "one-actor", RateLimit{Maximum: 7, Window: time.Minute})
			if err != nil {
				t.Errorf("allow: %v", err)
				return
			}
			if decision.Allowed {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := allowed.Load(); got != 7 {
		t.Fatalf("expected exactly 7 allowed calls, got %d", got)
	}
}
