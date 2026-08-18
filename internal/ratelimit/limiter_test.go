package ratelimit

import (
	"sync"
	"testing"
	"time"
)

func TestLimiterConsumesCapacityAndRefills(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	limiter := New(2, 1)
	limiter.Now = func() time.Time { return now }
	if first := limiter.Allow("operator"); !first.Allowed || first.Remaining != 1 {
		t.Fatalf("first decision %+v", first)
	}
	if second := limiter.Allow("operator"); !second.Allowed || second.Remaining != 0 {
		t.Fatalf("second decision %+v", second)
	}
	blocked := limiter.Allow("operator")
	if blocked.Allowed || blocked.RetryAfter < time.Second {
		t.Fatalf("blocked decision %+v", blocked)
	}
	now = now.Add(1100 * time.Millisecond)
	if refill := limiter.Allow("operator"); !refill.Allowed {
		t.Fatalf("refill decision %+v", refill)
	}
}

func TestLimiterSeparatesKeysAndCleansIdleBuckets(t *testing.T) {
	now := time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)
	limiter := New(1, 1)
	limiter.Now = func() time.Time { return now }
	limiter.Allow("a")
	limiter.Allow("b")
	if limiter.Size() != 2 {
		t.Fatalf("size = %d", limiter.Size())
	}
	now = now.Add(2 * time.Minute)
	if removed := limiter.Cleanup(time.Minute); removed != 2 {
		t.Fatalf("removed = %d", removed)
	}
	if limiter.Size() != 0 {
		t.Fatalf("size after cleanup = %d", limiter.Size())
	}
}

func TestLimiterIsSafeForConcurrentCallers(t *testing.T) {
	limiter := New(100, 1)
	var group sync.WaitGroup
	for i := 0; i < 50; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 20; j++ {
				limiter.Allow("same")
			}
		}()
	}
	group.Wait()
	if limiter.Size() != 1 {
		t.Fatalf("unexpected bucket count %d", limiter.Size())
	}
}
