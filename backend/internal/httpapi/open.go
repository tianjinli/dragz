package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
)

// Open API (/api/open/v1) for third-party callers that present a ua_* API token.
// The token's tenant_id is the tenant on the request. Scopes are the free-form
// JSON array on sys_api_token (see createAPITokenHandler):
//
//	open.me, open.tenant, open.ping, open (all three), * (every scope)
//
// Calls are metered by meterOpenAPI (see routes.go): one api_calls usage
// counter per tenant per day, enforced against the tenant's api_calls_per_day
// quota when the plan sets one.
const openAPIPrefix = "/api/open/"

const (
	openScopeAll    = "open"
	openScopeMe     = "open.me"
	openScopeTenant = "open.tenant"
	openScopePing   = "open.ping"
)

// openSkipAudit is the false Audit pointer for open routes. GET requests
// already skip the audit log; the pointer keeps a copied mutating route quiet.
var openSkipAudit bool

func isOpenAPIPath(path string) bool {
	return strings.HasPrefix(path, openAPIPrefix)
}

func isOpenScopeLiteral(scope string) bool {
	switch scope {
	case openScopeAll, openScopeMe, openScopeTenant, openScopePing:
		return true
	default:
		return false
	}
}

// requireOpenScope allows the request when the API token scopes contain any
// required scope, the blanket scope "open", or "*". A missing or empty scope
// list means the caller is a JWT session (or a token with no scopes).
func requireOpenScope(required []string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		scopes, ok := tokenScopesFrom(r)
		if !ok || len(scopes) == 0 {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.open_token_required")
			return
		}
		if !openScopeAllows(scopes, required) {
			fail(w, r, http.StatusForbidden, 40301, "errors.token_scope")
			return
		}
		next(w, r)
	}
}

func tokenScopesFrom(r *http.Request) ([]string, bool) {
	value := r.Context().Value(ctxkey.TokenScopes)
	if value == nil {
		return nil, false
	}
	scopes, ok := value.([]string)
	return scopes, ok
}

func openScopeAllows(scopes, required []string) bool {
	for _, scope := range scopes {
		if scope == "*" || scope == openScopeAll {
			return true
		}
		for _, want := range required {
			if scope == want {
				return true
			}
		}
	}
	return false
}

type openMeResponse struct {
	UserID   uint64   `json:"userId"`
	Username string   `json:"username"`
	TenantID uint64   `json:"tenantId"`
	Scopes   []string `json:"scopes"`
}

type openTenantResponse struct {
	ID     uint64 `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status int16  `json:"status"`
}

type openPingResponse struct {
	OK       bool   `json:"ok"`
	TenantID uint64 `json:"tenantId"`
}

func openMeHandler(*app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, okUser := currentUserID(r.Context())
		if !okUser {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.invalid_session")
			return
		}
		tenantID, _ := ctxkey.TenantIDFrom(r.Context())
		scopes, _ := tokenScopesFrom(r)
		if len(scopes) == 0 {
			scopes = []string{}
		} else {
			scopes = append([]string(nil), scopes...)
		}
		ok(w, r, openMeResponse{
			UserID:   userID,
			Username: currentUsername(r.Context()),
			TenantID: tenantID,
			Scopes:   scopes,
		})
	}
}

func openTenantHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := ctxkey.TenantIDFrom(r.Context())
		if tenantID == 0 {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if ctx == nil || ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		item, err := ctx.Tenants.GetOpenTenant(r.Context(), tenantID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
				return
			}
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			return
		}
		ok(w, r, openTenantResponse{ID: item.ID, Code: item.Code, Name: item.Name, Status: item.Status})
	}
}

func openPingHandler(*app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, _ := ctxkey.TenantIDFrom(r.Context())
		ok(w, r, openPingResponse{OK: true, TenantID: tenantID})
	}
}
