package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

// hostTenantHook runs on every route, after Localize and before Gate.
// A platform host, or a host that does not resolve, leaves the context alone.
func hostTenantHook(appCtx *app.Context) func(next http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if appCtx == nil || appCtx.Tenants == nil {
				next(w, r)
				return
			}
			host := store.NormalizeHost(r.Host)
			if host == "" || isPlatformHost(appCtx, host) {
				next(w, r)
				return
			}
			id, _, _, ok, err := appCtx.Tenants.ResolveByHost(r.Context(), host)
			if err != nil {
				fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
				return
			}
			if !ok || id == 0 {
				next(w, r)
				return
			}
			next(w, r.WithContext(ctxkey.WithHostTenantID(r.Context(), id)))
		}
	}
}

// isPlatformHost is true for configured platform hosts and for any raw IP.
// Those requests do not force a tenant.
func isPlatformHost(appCtx *app.Context, host string) bool {
	host = store.NormalizeHost(host)
	if host == "" || net.ParseIP(host) != nil {
		return true
	}
	if host == "localhost" || host == "127.0.0.1" {
		return true
	}
	if appCtx == nil {
		return false
	}
	for _, item := range appCtx.Config.PlatformHosts {
		if store.NormalizeHost(item) == host {
			return true
		}
	}
	return false
}

type tenantContextResponse struct {
	Resolved bool   `json:"resolved"`
	TenantID uint64 `json:"tenantId"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
}

func tenantContextHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, present := ctxkey.HostTenantIDFrom(r.Context())
		if !present || id == 0 || ctx == nil || ctx.Tenants == nil {
			ok(w, r, tenantContextResponse{})
			return
		}
		resolvedID, slug, name, found, err := ctx.Tenants.ResolveByHost(r.Context(), r.Host)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.tenant_load_failed")
			return
		}
		if !found {
			ok(w, r, tenantContextResponse{Resolved: true, TenantID: id})
			return
		}
		ok(w, r, tenantContextResponse{Resolved: true, TenantID: resolvedID, Slug: slug, Name: name})
	}
}

type setTenantSlugRequest struct {
	Slug string `json:"slug"`
}

type addTenantDomainRequest struct {
	Host string `json:"host"`
}

func setPlatformTenantSlugHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req setTenantSlugRequest
		if !decode(w, r, &req) {
			return
		}
		slug := strings.ToLower(strings.TrimSpace(req.Slug))
		if message := slugErrorKey(slug); message != "" {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, message)
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.SetSlug(r.Context(), id, slug)
		if writeTenantMutationError(w, r, err) {
			return
		}
		ok(w, r, map[string]any{"success": true, "slug": slug})
	}
}

func listPlatformTenantDomainsHandler(ctx *app.Context) http.HandlerFunc {
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
		items, err := ctx.Tenants.ListDomains(r.Context(), id)
		if writeTenantMutationError(w, r, err) {
			return
		}
		ok(w, r, items)
	}
}

func addPlatformTenantDomainHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req addTenantDomainRequest
		if !decode(w, r, &req) {
			return
		}
		host := store.NormalizeHost(req.Host)
		if !store.ValidDomainHost(host) || isPlatformHost(ctx, host) {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_domain_invalid")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		item, err := ctx.Tenants.AddDomain(r.Context(), id, host)
		if writeTenantMutationError(w, r, err) {
			return
		}
		ok(w, r, item)
	}
}

func verifyPlatformTenantDomainHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, domainID, valid := tenantAndDomainID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.MarkDomainVerified(r.Context(), tenantID, domainID)
		if writeTenantMutationError(w, r, err) {
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func removePlatformTenantDomainHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, domainID, valid := tenantAndDomainID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.RemoveDomain(r.Context(), tenantID, domainID)
		if writeTenantMutationError(w, r, err) {
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func tenantAndDomainID(r *http.Request) (uint64, uint64, bool) {
	tenantID, tenantOK := pathID(r)
	domainID, domainOK := pathUint(r, "domainId")
	return tenantID, domainID, tenantOK && domainOK
}

func pathUint(r *http.Request, name string) (uint64, bool) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(pathvar.Vars(r)[name]), 10, 64)
	return parsed, err == nil && parsed > 0
}

func slugErrorKey(slug string) string {
	switch store.SlugRejection(slug) {
	case "reserved":
		return "errors.tenant_slug_reserved"
	case "invalid":
		return "errors.tenant_slug_invalid"
	default:
		return ""
	}
}

// writeTenantMutationError maps store errors for slug and domain writes.
// true means a response was written.
func writeTenantMutationError(w http.ResponseWriter, r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
	case errors.Is(err, store.ErrDuplicateEntry):
		name := store.ConstraintName(err)
		switch {
		case strings.Contains(name, "slug"):
			fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.tenant_slug_taken")
		case strings.Contains(name, "domain"), strings.Contains(name, "host"):
			fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.tenant_domain_taken")
		default:
			fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.tenant_code_taken")
		}
	default:
		fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
	}
	return true
}
