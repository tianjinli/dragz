package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

// wsHandler upgrades a notification socket. Browsers cannot set
// Authorization on a WebSocket, so the access token arrives as ?token=
// and is checked here. The route is public so the Bearer middleware does
// not reject the handshake before this runs.
//
// The token names the tenant, and the socket is bound to it. Membership is
// re-checked here because a socket outlives the response that authorised it: a
// member removed from the tenant must not keep receiving that tenant's events
// just because their access token has not expired yet.
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
		if claims.TenantID == 0 {
			// A realtime channel without a tenant would be a cross-tenant
			// broadcast; refuse instead of defaulting.
			fail(w, r, http.StatusUnauthorized, 40100, "errors.tenant_required")
			return
		}
		wsCtx := ctxkey.WithTenantID(r.Context(), claims.TenantID)
		user, err := ctx.Auth.FindUserByID(wsCtx, claims.UserID)
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
		// The websocket handshake is a public route, so the HTTP guard never
		// sees it. Keep it inside the same first-login restriction.
		if user.MustChangePassword {
			fail(w, r, http.StatusForbidden, apierrors.CodePasswordChangeRequired, "errors.password_change_required")
			return
		}
		if _, err := ctx.Tenants.Membership(wsCtx, claims.UserID, claims.TenantID); err != nil {
			if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrNotFound) {
				fail(w, r, http.StatusForbidden, 40300, "errors.tenant_forbidden")
				return
			}
			fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
			return
		}
		enabled, err := ctx.Auth.FeatureEnabled(wsCtx, "feature.notice.enabled", true)
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
		ctx.WS.HandleHTTP(w, r, claims.TenantID, claims.UserID)
	}
}
