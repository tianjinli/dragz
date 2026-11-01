package httpapi

import (
	"net/http"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
)

func requirePermission(ctx *app.Context, permission string, next http.HandlerFunc) http.HandlerFunc {
	return requireAnyPermission(ctx, []string{permission}, next)
}

func requireAnyPermission(ctx *app.Context, permissions []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil || user.Status != 1 {
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		if user.IsSuperAdmin {
			if !tokenScopeAllowsAny(r, permissions) {
				fail(w, r, http.StatusForbidden, 40301, "errors.token_scope")
				return
			}
			next(w, r)
			return
		}
		scopeAllowed := tokenScopeAllowsAny(r, permissions)
		for _, permission := range permissions {
			allowed, err := ctx.PermissionAllowed(r.Context(), userID, false, permission)
			if err != nil {
				fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
				return
			}
			if allowed && scopeAllowed {
				next(w, r)
				return
			}
		}
		fail(w, r, http.StatusForbidden, 40300, "errors.forbidden")
	}
}

func tokenScopeAllowsAny(r *http.Request, permissions []string) bool {
	for _, permission := range permissions {
		if tokenScopeAllows(r, permission) {
			return true
		}
	}
	return false
}

func tokenScopeAllows(r *http.Request, permission string) bool {
	value := r.Context().Value(ctxkey.TokenScopes)
	if value == nil {
		return true
	}
	scopes, valid := value.([]string)
	if !valid {
		return false
	}
	for _, scope := range scopes {
		if scope == "*" || scope == permission {
			return true
		}
	}
	return false
}
