package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

// tenantHook is the middleware the kernel runs immediately after Authenticate.
// It turns the tenant id carried by the token into a verified membership.
func tenantHook(ctx *app.Context) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return resolveTenant(ctx, next)
	}
}

func resolveTenant(appCtx *app.Context, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r.Context())
		if !ok {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.invalid_session")
			return
		}
		claimed, _ := ctxkey.TenantIDFrom(r.Context())
		membership, found, err := lookupMembership(r.Context(), appCtx, userID, claimed)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			return
		}
		tenantID := uint64(0)
		operator := false
		if found {
			tenantID = membership.ID
			operator = membership.PlatformOperator
		} else if claimed != 0 {
			// Platform operators (and super administrators) may hold an
			// impersonation token for a tenant they are not a member of.
			enter, enterErr := platformMayEnterTenant(r.Context(), appCtx, userID, claimed)
			if enterErr != nil {
				fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
				return
			}
			if !enter {
				fail(w, r, http.StatusForbidden, apierrors.CodeTenantForbidden, "errors.tenant_forbidden")
				return
			}
			tenantID = claimed
			operator = true
		}
		if hostTid, hostOK := ctxkey.HostTenantIDFrom(r.Context()); hostOK && hostTid != 0 && tenantID != 0 && hostTid != tenantID {
			fail(w, r, http.StatusConflict, apierrors.CodeTenantHostMismatch, "errors.tenant_host_mismatch")
			return
		}
		reqCtx := ctxkey.WithTenantID(r.Context(), tenantID)
		reqCtx = ctxkey.WithPlatformOperator(reqCtx, operator)
		if appCtx.DB == nil {
			next(w, r.WithContext(reqCtx))
			return
		}
		tx, err := appCtx.DB.BeginTx(reqCtx, nil)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			return
		}
		reqCtx = ctxkey.WithTx(reqCtx, tx.Tx)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if rec := recover(); rec != nil {
				_ = tx.Rollback()
				panic(rec)
			}
		}()
		next(sw, r.WithContext(reqCtx))
		if sw.status >= 500 {
			_ = tx.Rollback()
			return
		}
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			if !sw.wrote {
				fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			}
		}
	}
}

// statusWriter records the status the handler wrote so the request transaction
// can roll back on a server error and commit otherwise.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response does not support hijacking")
	}
	return h.Hijack()
}

// platformMayEnterTenant is the impersonation fallback: a live platform
// operator (or super administrator) with an open impersonation grant may act
// inside a tenant they do not belong to. Super administrators may also enter
// when the tenant exists, which keeps the bootstrap admin usable before any
// impersonation row is written.
func platformMayEnterTenant(ctx context.Context, appCtx *app.Context, userID, tenantID uint64) (bool, error) {
	if appCtx == nil || appCtx.Auth == nil || appCtx.Tenants == nil || tenantID == 0 {
		return false, nil
	}
	user, err := appCtx.Auth.FindUserByID(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if user.Status != 1 {
		return false, nil
	}
	if user.IsSuperAdmin {
		return appCtx.Tenants.Exists(ctx, tenantID)
	}
	ok, err := appCtx.Tenants.HasActiveImpersonation(ctx, tenantID, userID)
	if err != nil || !ok {
		return false, err
	}
	return appCtx.Tenants.Exists(ctx, tenantID)
}

// lookupMembership resolves claimed when it is non-zero, and the user's first
// membership when it is zero. found is false when the user has no matching row.
func lookupMembership(ctx context.Context, appCtx *app.Context, userID, claimed uint64) (store.TenantMembership, bool, error) {
	if appCtx == nil || appCtx.Tenants == nil {
		return store.TenantMembership{}, false, nil
	}
	if claimed == 0 {
		row, err := appCtx.Tenants.FirstForUser(ctx, userID)
		if errors.Is(err, sql.ErrNoRows) {
			return store.TenantMembership{}, false, nil
		}
		if err != nil {
			return store.TenantMembership{}, false, err
		}
		return row, true, nil
	}
	row, err := appCtx.Tenants.Membership(ctx, userID, claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return store.TenantMembership{}, false, nil
	}
	if err != nil {
		return store.TenantMembership{}, false, err
	}
	return row, true, nil
}

// errHostTenantForbidden means the request host resolved to a tenant the user
// does not belong to. Login and refresh answer 403.
var errHostTenantForbidden = errors.New("host tenant forbidden")

// issueTenantID is the tenant written into a freshly issued token. Zero means
// the user has no membership yet and stays on the platform domain.
// When the host resolved to a tenant, the token uses that tenant and a
// non-member is errHostTenantForbidden.
func issueTenantID(ctx context.Context, appCtx *app.Context, userID uint64) (uint64, error) {
	if hostTid, ok := ctxkey.HostTenantIDFrom(ctx); ok && hostTid != 0 {
		if appCtx == nil || appCtx.Tenants == nil {
			return 0, errHostTenantForbidden
		}
		row, err := appCtx.Tenants.Membership(ctx, userID, hostTid)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, errHostTenantForbidden
		}
		if err != nil {
			return 0, err
		}
		return row.ID, nil
	}
	row, found, err := lookupMembership(ctx, appCtx, userID, 0)
	if err != nil || !found {
		return 0, err
	}
	return row.ID, nil
}

// refreshTenantID decides which tenant a refreshed session continues in.
//
// The tenant recorded on the refresh token wins, but only while the membership
// is still active. A user who belongs to two tenants switched to the second one;
// re-deriving the tenant from the host or from "first membership" would drag
// them back to the first on every refresh. A removed member, on the other hand,
// must not have that tenant renewed, so a dead membership falls through to a
// fresh resolution — which will fail if they have no other tenant.
func refreshTenantID(ctx context.Context, appCtx *app.Context, session store.RefreshSession) (uint64, error) {
	if session.TenantID.Valid && session.TenantID.Int64 > 0 {
		tenantID := uint64(session.TenantID.Int64)
		if appCtx != nil && appCtx.Tenants != nil {
			_, err := appCtx.Tenants.Membership(ctx, session.UserID, tenantID)
			switch {
			case err == nil:
				return tenantID, nil
			case errors.Is(err, sql.ErrNoRows), errors.Is(err, store.ErrNotFound):
				// Membership is gone: fall through and resolve again.
			default:
				return 0, err
			}
		}
	}
	return issueTenantID(ctx, appCtx, session.UserID)
}

// failIssuedTenant writes the HTTP error for issueTenantID. false means the
// caller should continue with the returned tenant id.
func failIssuedTenant(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, errHostTenantForbidden) {
		fail(w, r, http.StatusForbidden, apierrors.CodeTenantForbidden, "errors.tenant_forbidden")
		return true
	}
	fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
	return true
}

func withClaimedTenant(r *http.Request, userID uint64, username string, tenantID uint64) *http.Request {
	request := withUserContext(r, userID, username)
	return request.WithContext(ctxkey.WithTenantID(request.Context(), tenantID))
}

type switchTenantRequest struct {
	TenantID uint64 `json:"tenantId"`
}

type profileTenant struct {
	ID               uint64 `json:"id"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	PlatformOperator bool   `json:"platformOperator"`
}

func switchTenantHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.invalid_session")
			return
		}
		var req switchTenantRequest
		if !decode(w, r, &req) {
			return
		}
		if req.TenantID == 0 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			return
		}
		if _, err := ctx.Tenants.Membership(r.Context(), userID, req.TenantID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(w, r, http.StatusForbidden, apierrors.CodeTenantForbidden, "errors.tenant_forbidden")
				return
			}
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			return
		}
		if hostTid, hostOK := ctxkey.HostTenantIDFrom(r.Context()); hostOK && hostTid != 0 && hostTid != req.TenantID {
			fail(w, r, http.StatusConflict, apierrors.CodeTenantHostMismatch, "errors.tenant_host_mismatch")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil || user.Status != 1 {
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		pair, err := ctx.Tokens.Issue(user.ID, user.Username, user.TokenVersion, req.TenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.token_issue_failed")
			return
		}
		// Record the tenant the switch moved to: the next refresh must stay
		// here instead of resolving the user's first membership again.
		if err := ctx.Auth.SaveRefreshToken(r.Context(), user.ID, req.TenantID, pair.RefreshHash, pair.RefreshFamilyID,
			"", "", clientIP(r), r.UserAgent(), pair.RefreshExpiresAt); err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.session_create_failed")
			return
		}
		writeRefreshCookie(w, ctx, pair.RefreshToken)
		ok(w, r, pairResponse(ctx, pair))
	}
}

func profileTenants(r *http.Request, ctx *app.Context, userID uint64) (uint64, []profileTenant, error) {
	tenantID, _ := ctxkey.TenantIDFrom(r.Context())
	if ctx.Tenants == nil {
		return tenantID, []profileTenant{}, nil
	}
	rows, err := ctx.Tenants.ListForUser(r.Context(), userID)
	if err != nil {
		return 0, nil, err
	}
	items := make([]profileTenant, 0, len(rows))
	for _, row := range rows {
		items = append(items, profileTenant{
			ID: row.ID, Code: row.Code, Name: row.Name, PlatformOperator: row.PlatformOperator,
		})
	}
	return tenantID, items, nil
}
