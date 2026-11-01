package httpapi

import (
	"net/http"

	"dragz-admin/backend/internal/app"
)

func requireFeature(ctx *app.Context, key string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, err := ctx.Auth.FeatureEnabled(r.Context(), key, true)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.feature_read_failed")
			return
		}
		if !enabled {
			fail(w, r, http.StatusServiceUnavailable, 50301, "errors.feature_disabled")
			return
		}
		next(w, r)
	}
}
