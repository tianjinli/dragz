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
