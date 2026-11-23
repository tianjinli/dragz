package httpapi

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
)

// usageAPIKey is the metered counter. The period is the UTC day, so a limit
// expressed as "calls per day" resets at a predictable instant for every
// deployment regardless of server timezone.
const (
	usageAPIKey    = "api_calls"
	quotaAPIKey    = "api_calls_per_day"
	usageAPIPeriod = "2006-01-02"
)

// meterOpenAPI counts one billed API call and enforces the daily quota.
//
// Metering failures are logged, not returned: an outage of the counter must not
// turn a working integration into a 500. The quota check, once the counter row
// exists, is authoritative.
func meterOpenAPI(appCtx *app.Context, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		if !okTenant || tenantID == 0 || appCtx.Tenants == nil {
			next(w, r)
			return
		}
		period := time.Now().UTC().Format(usageAPIPeriod)
		used, err := appCtx.Tenants.AddUsage(r.Context(), tenantID, usageAPIKey, period, 1)
		if err != nil {
			log.Printf("api usage meter: tenant %d: %v", tenantID, err)
			next(w, r)
			return
		}
		limit, configured, err := appCtx.Tenants.QuotaLimit(r.Context(), tenantID, quotaAPIKey)
		if err != nil {
			log.Printf("api usage quota: tenant %d: %v", tenantID, err)
			next(w, r)
			return
		}
		if configured && limit > 0 && used > limit {
			w.Header().Set("Retry-After", secondsUntilUTCMidnight())
			fail(w, r, http.StatusTooManyRequests, apierrors.CodeRateLimited, "errors.quota_exceeded")
			return
		}
		next(w, r)
	}
}

func secondsUntilUTCMidnight() string {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Add(24 * time.Hour)
	seconds := int(next.Sub(now).Seconds()) + 1
	if seconds <= 0 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}
