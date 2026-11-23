// Package fairgate bounds the concurrent scheduled work of one tenant.
//
// Cron jobs are enqueued by a single process-level scheduler, so without a gate
// one tenant that schedules a slow job at :00 can occupy every worker and delay
// every other tenant's :00 job. The gate is deliberately crude — a Redis
// counter per tenant — because it only has to stop starvation, not schedule
// work fairly at millisecond granularity.
//
// A slot is held for the duration of one run and released in a defer. The
// counter also carries a TTL so a worker that dies without releasing cannot
// block its tenant forever.
package fairgate

import (
	"context"
	"fmt"
	"time"

	redisstore "github.com/zeromicro/go-zero/core/stores/redis"
)

// script acquires one slot. INCR and EXPIRE are one round trip so a crash
// between them cannot strand a counter without a TTL.
const script = `
local current = redis.call('INCR', KEYS[1])
if current > tonumber(ARGV[1]) then
  redis.call('DECR', KEYS[1])
  return 0
end
if current == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 1
`

// Gate admits at most Limit concurrent runs per tenant.
type Gate struct {
	cache *redisstore.Redis
	limit int
	ttl   time.Duration
}

// New builds a gate. A nil cache or a non-positive limit disables admission
// control, which is what tests and single-tenant deployments want.
func New(cache *redisstore.Redis, limit int, ttl time.Duration) *Gate {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Gate{cache: cache, limit: limit, ttl: ttl}
}

// Enabled reports whether the gate actually enforces anything.
func (g *Gate) Enabled() bool {
	return g != nil && g.cache != nil && g.limit > 0
}

// Acquire takes one slot for tenantID. When ok is false the caller must not run
// the job now and should defer it. release is always non-nil.
func (g *Gate) Acquire(ctx context.Context, tenantID uint64) (release func(), ok bool) {
	if !g.Enabled() || tenantID == 0 {
		return func() {}, true
	}
	key := fmt.Sprintf("jobgate:tenant:%d", tenantID)
	value, err := g.cache.EvalCtx(ctx, script, []string{key},
		g.limit, g.ttl.Milliseconds())
	if err != nil {
		// The gate is a fairness mechanism, not a correctness one: if Redis is
		// unreachable, running the job beats dropping it.
		return func() {}, true
	}
	if n, valid := value.(int64); valid && n == 0 {
		return func() {}, false
	}
	released := false
	return func() {
		if released {
			return
		}
		released = true
		_, _ = g.cache.DecrCtx(context.Background(), key)
	}, true
}
