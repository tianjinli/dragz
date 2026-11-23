package httpapi

import (
	"net/http"

	"dragz-admin/backend/internal/app"
)

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok(w, r, map[string]string{"status": "ok"})
	}
}

// metricsHandler serves the Prometheus text exposition of the request meter.
// The registry answers 404 itself when metrics.enabled is off, so this stays a
// one-liner and there is no second place to check the switch.
func metricsHandler(ctx *app.Context) http.HandlerFunc {
	return ctx.Metrics.Handler()
}

func readyHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := ctx.DB.PingContext(r.Context()); err != nil {
			fail(w, r, http.StatusServiceUnavailable, 50300, "errors.internal")
			return
		}
		if ctx.Cache != nil {
			if !ctx.Cache.Ping() {
				fail(w, r, http.StatusServiceUnavailable, 50300, "errors.internal")
				return
			}
		}
		ok(w, r, map[string]string{"status": "ready"})
	}
}
