package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
)

// wsHandler upgrades a notification socket. Browsers cannot set
// Authorization on a WebSocket, so the access token arrives as ?token=
// and is checked here. The route is public so the Bearer middleware does
// not reject the handshake before this runs.
func wsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" || ctx.Tokens == nil {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.unauthorized")
			return
		}
		claims, err := ctx.Tokens.ValidateAccess(token)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.unauthorized")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), claims.UserID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(w, r, http.StatusUnauthorized, 40100, "errors.unauthorized")
				return
			}
			fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
			return
		}
		if user.Status != 1 || user.TokenVersion != claims.TokenVersion {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.unauthorized")
			return
		}
		enabled, err := ctx.Auth.FeatureEnabled(r.Context(), "feature.notice.enabled", true)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.feature_read_failed")
			return
		}
		if !enabled {
			fail(w, r, http.StatusServiceUnavailable, 50301, "errors.feature_disabled")
			return
		}
		if ctx.WS == nil {
			fail(w, r, http.StatusServiceUnavailable, 50301, "errors.ws_unavailable")
			return
		}
		ctx.WS.HandleHTTP(w, r, claims.UserID)
	}
}
