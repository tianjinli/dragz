package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/security"
)

func authenticate(ctx *app.Context, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := strings.TrimSpace(r.Header.Get("Authorization"))
		if len(header) < 8 || !strings.EqualFold(header[:7], "Bearer ") {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.unauthorized")
			return
		}
		value := strings.TrimSpace(header[7:])
		if strings.HasPrefix(value, "ua_") {
			principal, err := ctx.Auth.AuthenticateAPIToken(r.Context(), security.HashOpaqueToken(value), clientIP(r))
			if err != nil {
				fail(w, r, http.StatusUnauthorized, 40106, "errors.api_token_invalid")
				return
			}
			request := withUserContext(r, principal.UserID, principal.Username)
			request = withTokenScopes(request, principal.Scopes)
			next(w, request)
			return
		}

		claims, err := ctx.Tokens.ValidateAccess(value)
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
		next(w, withUserContext(r, claims.UserID, claims.Username))
	}
}
