package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
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
			claimed := principal.TenantID
			if claimed == 0 {
				claimed, err = apiTokenTenantID(ctx, r, principal.TokenID)
				if err != nil {
					fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
					return
				}
			}
			request := withClaimedTenant(r, principal.UserID, principal.Username, claimed)
			request = withTokenScopes(request, principal.Scopes)
			next(w, request)
			return
		}

		// Open routes accept only API tokens. Any other bearer (a JWT session,
		// or a token that never produced scopes) stops here.
		if isOpenAPIPath(r.URL.Path) {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.open_token_required")
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
		next(w, withClaimedTenant(r, claims.UserID, claims.Username, claims.TenantID))
	}
}

// apiTokenTenantID uses the token's tenant_id column when the database has
// one. A missing column, or a stored zero, leaves the claim at zero so the
// tenant middleware falls back to the user's first membership.
func apiTokenTenantID(ctx *app.Context, r *http.Request, tokenID uint64) (uint64, error) {
	if ctx.Tenants == nil {
		return 0, nil
	}
	id, exists, err := ctx.Tenants.APITokenTenantID(r.Context(), tokenID)
	if err != nil || !exists {
		return 0, err
	}
	return id, nil
}
