package httpapi

import (
	"net/http"
	"strconv"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/plugin"
)

// Default per-minute budgets. They are deliberately generous: the limiter is
// a safety net against floods and credential stuffing, not a usage plan. The
// metered API quota is a separate mechanism (see the open API handlers).
const (
	// anonymousRateLimit is the default for public routes that do not set
	// their own budget.
	anonymousRateLimit = 120
	// authenticatedRateLimit is the default for a signed-in client. A dashboard
	// polling a handful of endpoints fits comfortably; an export loop does not.
	authenticatedRateLimit = 600
	// loginRateLimit is for endpoints whose cost is "check a password". Ten
	// attempts a minute per address still leaves room for a typo and a retry.
	loginRateLimit = 10
	// sensitiveRateLimit is for endpoints that write or email something:
	// signup, invitations, password reset.
	sensitiveRateLimit = 20
)

// rateLimitHook builds the limiter middleware. It runs after authentication and
// tenant resolution, so a signed-in caller is keyed by account rather than by
// address (one NAT must not throttle a whole office), and it runs before Audit
// so a rejected flood does not become a flood of audit rows.
func rateLimitHook(ctx *app.Context) func(route plugin.Route, next http.HandlerFunc) http.HandlerFunc {
	return func(route plugin.Route, next http.HandlerFunc) http.HandlerFunc {
		budget := route.RateLimit
		if budget <= 0 {
			if route.Public {
				budget = anonymousRateLimit
			} else {
				budget = authenticatedRateLimit
			}
		}
		routeKey := route.Method + " " + route.Path
		return func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter := ctx.RateLimit.Allow(r.Context(), rateLimitKey(r, routeKey), budget)
			if allowed {
				next(w, r)
				return
			}
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			fail(w, r, http.StatusTooManyRequests, apierrors.CodeRateLimited, "errors.rate_limited")
		}
	}
}

// rateLimitKey identifies the caller for one route. An authenticated caller is
// keyed by account and tenant; an anonymous one by address.
func rateLimitKey(r *http.Request, routeKey string) string {
	if userID, ok := currentUserID(r.Context()); ok && userID != 0 {
		tenantID, _ := ctxkey.TenantIDFrom(r.Context())
		return routeKey + "|u:" + strconv.FormatUint(userID, 10) +
			"|t:" + strconv.FormatUint(tenantID, 10)
	}
	return routeKey + "|ip:" + clientIP(r)
}
