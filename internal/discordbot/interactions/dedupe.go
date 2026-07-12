package interactions

import (
	"sync"
	"time"
)

// InteractionDeduper atomically claims Discord interaction IDs for a bounded time window.
type InteractionDeduper struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	ttl     time.Duration
	maxSize int
	now     func() time.Time
}

// NewInteractionDeduper constructs a process-local duplicate boundary. Discord
// retries are short-lived and case creation also retains its durable idempotency key.
func NewInteractionDeduper(ttl time.Duration, maxSize int) *InteractionDeduper {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &InteractionDeduper{seen: map[string]time.Time{}, ttl: ttl, maxSize: maxSize, now: time.Now}
}

// Claim returns true exactly once for an interaction ID within the configured window.
func (d *InteractionDeduper) Claim(id string) bool {
	if d == nil || id == "" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now().UTC()
	if expires, ok := d.seen[id]; ok && expires.After(now) {
		return false
	}
	if len(d.seen) >= d.maxSize {
		for key, expires := range d.seen {
			if !expires.After(now) {
				delete(d.seen, key)
			}
		}
		if len(d.seen) >= d.maxSize {
			var oldestKey string
			var oldest time.Time
			for key, expires := range d.seen {
				if oldestKey == "" || expires.Before(oldest) {
					oldestKey, oldest = key, expires
				}
			}
			delete(d.seen, oldestKey)
		}
	}
	d.seen[id] = now.Add(d.ttl)
	return true
}
