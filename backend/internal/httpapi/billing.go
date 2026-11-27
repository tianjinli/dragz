package httpapi

import (
	"errors"
	"net/http"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

func exportPlatformTenantHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		// TODO: async zip export for large tenants. The MVP returns this JSON snapshot.
		snap, err := ctx.Tenants.ExportTenant(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, snap)
	}
}

type schedulePurgeRequest struct {
	Days int `json:"days"`
}

func schedulePlatformPurgeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req schedulePurgeRequest
		if !decode(w, r, &req) {
			return
		}
		if req.Days < 1 || req.Days > 3650 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.ScheduleTenantPurge(r.Context(), id, req.Days)
		if errors.Is(err, store.ErrProtected) {
			fail(w, r, http.StatusConflict, apierrors.CodeProtected, "errors.tenant_protected")
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]any{"success": true, "days": req.Days})
	}
}

func cancelPlatformPurgeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.CancelTenantPurge(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if errors.Is(err, store.ErrPurgeNotCancelable) {
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.purge_not_cancelable")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}
