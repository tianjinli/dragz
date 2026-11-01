package httpapi

import (
	"database/sql"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/security"
)

type createAPITokenRequest struct {
	UserID      uint64   `json:"userId"`
	Name        string   `json:"name"`
	Scopes      []string `json:"scopes"`
	IPAllowlist []string `json:"ipAllowlist"`
	ExpiresAt   string   `json:"expiresAt"`
}

func listSessionsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListSessions(r.Context(), query.Get("keyword"), page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.session_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func revokeSessionHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.session_id_invalid")
			return
		}
		if err := ctx.Auth.RevokeSession(r.Context(), id, "administrator_revoked"); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.session_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listAPITokensHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		var userID *uint64
		if value := queryUint64(query.Get("userId")); value > 0 {
			userID = &value
		}
		var status *int8
		if value := strings.TrimSpace(query.Get("status")); value != "" {
			parsed, parseErr := strconv.ParseInt(value, 10, 8)
			if parseErr != nil || (parsed != 0 && parsed != 1) {
				fail(w, r, http.StatusBadRequest, 40001, "errors.token_status_invalid")
				return
			}
			converted := int8(parsed)
			status = &converted
		}
		items, total, err := ctx.Auth.ListAPITokens(r.Context(), query.Get("keyword"), userID, status, page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.api_token_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func createAPITokenHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createAPITokenRequest
		if !decode(w, r, &req) {
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" || len(req.Name) > 128 || len(req.Scopes) == 0 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.api_token_fields_invalid")
			return
		}
		if req.UserID == 0 {
			req.UserID, _ = currentUserID(r.Context())
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), req.UserID)
		if errors.Is(err, sql.ErrNoRows) || user.Status != 1 {
			fail(w, r, http.StatusBadRequest, 40002, "errors.api_token_user_invalid")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.permission_load_failed")
			return
		}
		permissions, err := ctx.Auth.PermissionCodes(r.Context(), req.UserID, user.IsSuperAdmin)
		if err != nil || !scopesAllowed(req.Scopes, permissions, user.IsSuperAdmin) {
			fail(w, r, http.StatusBadRequest, 40003, "errors.api_token_scope_exceeded")
			return
		}
		for _, ip := range req.IPAllowlist {
			if net.ParseIP(strings.TrimSpace(ip)) == nil {
				fail(w, r, http.StatusBadRequest, 40004, "errors.api_token_ip_invalid")
				return
			}
		}
		var expiresAt *time.Time
		if strings.TrimSpace(req.ExpiresAt) != "" {
			value, parseErr := time.Parse(time.RFC3339, req.ExpiresAt)
			if parseErr != nil || !value.After(time.Now()) {
				fail(w, r, http.StatusBadRequest, 40005, "errors.api_token_expires_invalid")
				return
			}
			expiresAt = &value
		}
		plainToken, tokenHash, err := security.NewAPIToken()
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.api_token_generate_failed")
			return
		}
		id, err := ctx.Auth.CreateAPIToken(r.Context(), req.UserID, req.Name, plainToken[:11], tokenHash, req.Scopes, req.IPAllowlist, expiresAt)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.api_token_save_failed")
			return
		}
		ok(w, r, map[string]any{"id": id, "token": plainToken, "shownOnce": true})
	}
}

func revokeAPITokenHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.api_token_id_invalid")
			return
		}
		if err := ctx.Auth.RevokeAPIToken(r.Context(), id, nil); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.api_token_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listLoginLogsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		success, valid := optionalBool(query.Get("success"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.success_flag_invalid")
			return
		}
		items, total, err := ctx.Auth.ListLoginLogs(r.Context(), query.Get("keyword"), success, page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.login_log_list_failed")
			return
		}
		// failure_reason holds a message ID; rows written before the switch hold
		// plain text, which localizedStored() returns unchanged.
		for i := range items {
			items[i].FailureReason = localizedStored(r, items[i].FailureReason)
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func listOperationLogsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		success, valid := optionalBool(query.Get("success"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.success_flag_invalid")
			return
		}
		items, total, err := ctx.Auth.ListOperationLogs(r.Context(), query.Get("keyword"), query.Get("module"), success, page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oper_log_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func listErrorLogsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		resolved, valid := optionalBool(query.Get("resolved"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.handled_flag_invalid")
			return
		}
		items, total, err := ctx.Auth.ListErrorLogs(r.Context(), query.Get("keyword"), resolved, page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.error_log_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func resolveErrorLogHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.error_log_id_invalid")
			return
		}
		userID, _ := currentUserID(r.Context())
		if err := ctx.Auth.ResolveErrorLog(r.Context(), id, userID); err != nil {
			fail(w, r, http.StatusConflict, 40901, "errors.error_log_handled")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func optionalBool(value string) (*bool, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	parsed, err := strconv.ParseBool(value)
	return &parsed, err == nil
}

func scopesAllowed(scopes, permissions []string, superAdmin bool) bool {
	available := make(map[string]struct{}, len(permissions))
	for _, permission := range permissions {
		available[permission] = struct{}{}
	}
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == "" {
			return false
		}
		if scope == "*" && superAdmin {
			continue
		}
		if _, exists := available[scope]; !exists {
			return false
		}
	}
	return true
}
