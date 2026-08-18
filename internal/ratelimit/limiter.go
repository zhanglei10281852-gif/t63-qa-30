package ratelimit

import (
	"sync"
	"time"
)

type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
}
type bucket struct {
	Tokens    float64
	UpdatedAt time.Time
	LastSeen  time.Time
}
type Limiter struct {
	mutex           sync.Mutex
	buckets         map[string]bucket
	Capacity        int
	RefillPerSecond float64
	Now             func() time.Time
}

func New(capacity int, refillPerSecond float64) *Limiter {
	if capacity <= 0 {
		capacity = 60
	}
	if refillPerSecond <= 0 {
		refillPerSecond = 1
	}
	return &Limiter{buckets: map[string]bucket{}, Capacity: capacity, RefillPerSecond: refillPerSecond, Now: time.Now}
}

func (l *Limiter) Allow(key string) Decision {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	now := l.Now().UTC()
	value, exists := l.buckets[key]
	if !exists {
		value = bucket{Tokens: float64(l.Capacity), UpdatedAt: now}
	}
	elapsed := now.Sub(value.UpdatedAt).Seconds()
	if elapsed > 0 {
		value.Tokens += elapsed * l.RefillPerSecond
		if value.Tokens > float64(l.Capacity) {
			value.Tokens = float64(l.Capacity)
		}
	}
	value.UpdatedAt, value.LastSeen = now, now
	decision := Decision{}
	if value.Tokens >= 1 {
		value.Tokens--
		decision.Allowed = true
		decision.Remaining = int(value.Tokens)
	} else {
		missing := 1 - value.Tokens
		decision.RetryAfter = time.Duration(missing / l.RefillPerSecond * float64(time.Second))
		if decision.RetryAfter < time.Second {
			decision.RetryAfter = time.Second
		}
	}
	l.buckets[key] = value
	return decision
}

func (l *Limiter) Cleanup(maxIdle time.Duration) int {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	now := l.Now().UTC()
	removed := 0
	for key, value := range l.buckets {
		if now.Sub(value.LastSeen) > maxIdle {
			delete(l.buckets, key)
			removed++
		}
	}
	return removed
}
func (l *Limiter) Size() int { l.mutex.Lock(); defer l.mutex.Unlock(); return len(l.buckets) }
