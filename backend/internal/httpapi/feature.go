package httpapi

import (
	"net/http"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
)

func requireFeature(ctx *app.Context, key string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, err := ctx.Auth.FeatureEnabled(r.Context(), key, true)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.feature_read_failed")
			return
		}
		if !enabled {
			fail(w, r, http.StatusServiceUnavailable, apierrors.CodeFeatureDisabled, "errors.feature_disabled")
			return
		}
		tenantID, _ := ctxkey.TenantIDFrom(r.Context())
		if tenantID != 0 && ctx.Tenants != nil {
			entitled, err := ctx.Tenants.EntitlementEnabled(r.Context(), tenantID, key)
			if err != nil {
				fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.feature_read_failed")
				return
			}
			if !entitled {
				fail(w, r, http.StatusServiceUnavailable, apierrors.CodeFeatureDisabled, "errors.tenant_entitlement_disabled")
				return
			}
		}
		next(w, r)
	}
}
