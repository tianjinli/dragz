package httpapi

import (
	"net/http"

	"dragz-admin/backend/internal/app"
)

func dashboardHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := currentUserID(r.Context())
		summary, trend, err := ctx.Auth.Dashboard(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.dashboard_load_failed")
			return
		}
		ok(w, r, map[string]any{"summary": summary, "loginTrend": trend})
	}
}
