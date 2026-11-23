package httpapi

import (
	"errors"
	"net/http"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

func billingSummaryHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		if !okTenant || tenantID == 0 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		subscription, err := ctx.Tenants.GetSubscriptionDetail(r.Context(), tenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		quotas, err := ctx.Tenants.ListQuotasForTenant(r.Context(), tenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		if quotas == nil {
			quotas = []store.TenantQuota{}
		}
		invoices, err := ctx.Tenants.ListCurrentTenantInvoices(r.Context(), tenantID, 50)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		if invoices == nil {
			invoices = []store.PlatformInvoice{}
		}
		ok(w, r, store.BillingSummary{
			TenantID:     tenantID,
			Subscription: subscription,
			Quotas:       quotas,
			Invoices:     invoices,
		})
	}
}

// billingUsageHandler reports what the tenant has consumed. It is separate from
// the summary on purpose: the summary is read on every page load of the billing
// screen, while the counters are a heavier aggregate.
func billingUsageHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		if !okTenant || tenantID == 0 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		quotas, err := ctx.Tenants.ListQuotasForTenant(r.Context(), tenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		usage, err := ctx.Tenants.ListUsage(r.Context(), tenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		if quotas == nil {
			quotas = []store.TenantQuota{}
		}
		if usage == nil {
			usage = []store.UsageCounter{}
		}
		ok(w, r, map[string]any{"quotas": quotas, "usage": usage})
	}
}

func listPlatformInvoicesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		items, err := ctx.Tenants.ListRecentInvoices(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

func listPlatformTenantInvoicesHandler(ctx *app.Context) http.HandlerFunc {
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
		items, err := ctx.Tenants.ListTenantInvoices(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

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
