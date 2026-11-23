package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
)

type clearCacheRequest struct {
	Pattern      string `json:"pattern"`
	Confirmation string `json:"confirmation"`
}

type cacheKeyItem struct {
	Key          string `json:"key"`
	TTLSeconds   int    `json:"ttlSeconds"`
	ValuePreview string `json:"valuePreview"`
}

// cacheScanPattern turns the operator's search term into a scan pattern inside
// the caller's tenant namespace.
//
// The cache is shared by every tenant and also holds pre-authentication data
// (captcha answers, login failure counters). The administration endpoints must
// therefore never scan the keyspace directly: the pattern is always anchored at
// the tenant's own prefix, and a term that tries to escape it — a colon or a
// wildcard — is rejected instead of being pasted into the pattern.
//
// An empty term or "*" lists everything the tenant owns.
func cacheScanPattern(tenantID uint64, term string) (string, bool) {
	term = strings.TrimSpace(term)
	base := app.PermissionCacheKeyPrefix(tenantID)
	if term == "" || term == "*" {
		return base + "*", true
	}
	if strings.ContainsAny(term, ":*?[]") {
		return "", false
	}
	return base + "*" + term + "*", true
}

func tenantIDOf(r *http.Request) (uint64, bool) {
	tenantID, _ := ctxkey.TenantIDFrom(r.Context())
	return tenantID, tenantID != 0
}

func listCacheKeysHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := tenantIDOf(r)
		if !okTenant {
			fail(w, r, http.StatusForbidden, 40300, "errors.tenant_required")
			return
		}
		query := r.URL.Query()
		pattern, valid := cacheScanPattern(tenantID, query.Get("pattern"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.cache_pattern_invalid")
			return
		}
		if len(pattern) > 128 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.cache_pattern_too_long")
			return
		}
		cursor, _ := strconv.ParseUint(query.Get("cursor"), 10, 64)
		count := int64(queryInt(query.Get("pageSize"), 50))
		if count > 200 {
			count = 200
		}
		keys, nextCursor, err := ctx.Cache.ScanCtx(r.Context(), cursor, pattern, count)
		if err != nil {
			fail(w, r, http.StatusBadGateway, 50201, "errors.cache_read_failed")
			return
		}
		items := make([]cacheKeyItem, 0, len(keys))
		for _, key := range keys {
			ttl, _ := ctx.Cache.TtlCtx(r.Context(), key)
			value, _ := ctx.Cache.GetCtx(r.Context(), key)
			if len(value) > 512 {
				value = value[:512] + "…"
			}
			items = append(items, cacheKeyItem{Key: key, TTLSeconds: ttl, ValuePreview: value})
		}
		ok(w, r, map[string]any{"items": items, "nextCursor": nextCursor})
	}
}

func deleteCacheKeyHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := tenantIDOf(r)
		if !okTenant {
			fail(w, r, http.StatusForbidden, 40300, "errors.tenant_required")
			return
		}
		key := strings.TrimSpace(pathvarValue(r, "key"))
		if key == "" {
			fail(w, r, http.StatusBadRequest, 40001, "errors.cache_key_required")
			return
		}
		// A key outside the tenant namespace is reported as missing rather than
		// forbidden, so the endpoint cannot be used to probe other tenants.
		if !strings.HasPrefix(key, app.PermissionCacheKeyPrefix(tenantID)) {
			fail(w, r, http.StatusNotFound, 40400, "errors.cache_key_not_found")
			return
		}
		deleted, err := ctx.Cache.DelCtx(r.Context(), key)
		if err != nil {
			fail(w, r, http.StatusBadGateway, 50201, "errors.cache_delete_failed")
			return
		}
		ok(w, r, map[string]any{"deleted": deleted})
	}
}

func clearCacheHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := tenantIDOf(r)
		if !okTenant {
			fail(w, r, http.StatusForbidden, 40300, "errors.tenant_required")
			return
		}
		var req clearCacheRequest
		if !decode(w, r, &req) {
			return
		}
		pattern, valid := cacheScanPattern(tenantID, req.Pattern)
		if !valid || len(pattern) > 128 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.cache_pattern_invalid")
			return
		}
		// Clearing everything the tenant owns still needs an explicit
		// confirmation; the blast radius is one tenant, not the whole cache.
		if strings.TrimSpace(req.Pattern) == "*" && req.Confirmation != "DELETE ALL CACHE" {
			fail(w, r, http.StatusBadRequest, 40002, "errors.cache_clear_confirm")
			return
		}
		var cursor uint64
		deleted := 0
		for {
			keys, nextCursor, err := ctx.Cache.ScanCtx(r.Context(), cursor, pattern, 200)
			if err != nil {
				fail(w, r, http.StatusBadGateway, 50201, "errors.cache_scan_failed")
				return
			}
			if len(keys) > 0 {
				count, err := ctx.Cache.DelCtx(r.Context(), keys...)
				if err != nil {
					fail(w, r, http.StatusBadGateway, 50202, "errors.cache_batch_delete_failed")
					return
				}
				deleted += count
			}
			cursor = nextCursor
			if cursor == 0 || deleted >= 10_000 {
				break
			}
		}
		ok(w, r, map[string]any{"deleted": deleted, "limitReached": deleted >= 10_000})
	}
}
