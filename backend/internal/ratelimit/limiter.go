// Package ratelimit is a fixed-window request limiter shared by every route.
//
// It exists because the login and signup endpoints are reachable without a
// session: without a limiter, password guessing and signup floods are only
// bounded by how fast the attacker can open sockets. Authenticated routes get
// a much higher ceiling, mostly to stop a runaway client from pinning the
// database.
//
// The window is per minute and keyed by route plus caller. Redis holds the
// counter so the limit is shared by every API instance; when Redis is
// unavailable the limiter degrades to a per-process counter rather than
// failing requests, because an outage of the cache must not take the login
// page down with it.
package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	redisstore "github.com/zeromicro/go-zero/core/stores/redis"
)

// windowSeconds is the window length. It matches the per-minute budgets the
// route table declares.
const windowSeconds = 60

// incrScript increments the counter and sets the expiry in one round trip, so
// a crash between INCR and EXPIRE cannot leave a key that never resets.
const incrScript = `
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return current
`

// Limiter enforces per-minute budgets.
type Limiter struct {
	cache *redisstore.Redis

	mu    sync.Mutex
	local map[string]localWindow
}

type localWindow struct {
	window int64
	count  int
}

// New returns a limiter. A nil cache means the in-process counter is the only
// backend, which is what tests and single-process deployments use.
func New(cache *redisstore.Redis) *Limiter {
	return &Limiter{cache: cache, local: map[string]localWindow{}}
}

// Allow reports whether one more request fits the budget, plus how many
// seconds the caller should wait when it does not.
//
// key must already identify the caller; perMinute must be positive.
func (l *Limiter) Allow(ctx context.Context, key string, perMinute int) (bool, int) {
	if l == nil || perMinute <= 0 || key == "" {
		return true, 0
	}
	window := time.Now().Unix() / windowSeconds
	fullKey := fmt.Sprintf("ratelimit:%s:%d", key, window)
	if l.cache != nil {
		if value, err := l.cache.EvalCtx(ctx, incrScript, []string{fullKey}, windowSeconds*1000); err == nil {
			count, ok := toInt64(value)
			if ok {
				return count <= int64(perMinute), retryAfter()
			}
		}
	}
	count := l.localCount(fullKey, window)
	return count <= perMinute, retryAfter()
}

// localCount is the fallback counter. It also prunes expired windows so a
// long-running process does not accumulate one entry per caller per minute.
func (l *Limiter) localCount(key string, window int64) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.local) > 4096 {
		for k, entry := range l.local {
			if entry.window != window {
				delete(l.local, k)
			}
		}
	}
	entry := l.local[key]
	if entry.window != window {
		entry = localWindow{window: window}
	}
	entry.count++
	l.local[key] = entry
	return entry.count
}

func retryAfter() int {
	remaining := windowSeconds - int(time.Now().Unix()%windowSeconds)
	if remaining <= 0 {
		return windowSeconds
	}
	return remaining
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case []byte:
		parsed, err := strconv.ParseInt(string(v), 10, 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}
