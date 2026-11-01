package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"dragz-admin/backend/internal/app"
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

func listCacheKeysHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		pattern := strings.TrimSpace(query.Get("pattern"))
		if pattern == "" {
			pattern = "*"
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
		key := strings.TrimSpace(pathvarValue(r, "key"))
		if key == "" {
			fail(w, r, http.StatusBadRequest, 40001, "errors.cache_key_required")
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
		var req clearCacheRequest
		if !decode(w, r, &req) {
			return
		}
		req.Pattern = strings.TrimSpace(req.Pattern)
		if req.Pattern == "" || len(req.Pattern) > 128 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.cache_pattern_invalid")
			return
		}
		if req.Pattern == "*" && req.Confirmation != "DELETE ALL CACHE" {
			fail(w, r, http.StatusBadRequest, 40002, "errors.cache_clear_confirm")
			return
		}
		var cursor uint64
		deleted := 0
		for {
			keys, nextCursor, err := ctx.Cache.ScanCtx(r.Context(), cursor, req.Pattern, 200)
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
