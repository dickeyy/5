package interactions_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/quackdiscord/bot/internal/discordbot/interactions"
)

func TestInteractionDeduperClaimsOneConcurrentDelivery(t *testing.T) {
	deduper := interactions.NewInteractionDeduper(time.Minute, 100)
	var claimed atomic.Int64
	var wait sync.WaitGroup
	for range 100 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if deduper.Claim("interaction-1") {
				claimed.Add(1)
			}
		}()
	}
	wait.Wait()
	if claimed.Load() != 1 {
		t.Fatalf("expected exactly one claim, got %d", claimed.Load())
	}
}
