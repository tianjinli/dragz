package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/internal/store"
)

type auditResponseWriter struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (w *auditResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if len(w.body) < 64*1024 {
		remaining := 64*1024 - len(w.body)
		if remaining > len(value) {
			remaining = len(value)
		}
		w.body = append(w.body, value[:remaining]...)
	}
	return w.ResponseWriter.Write(value)
}

func auditRoute(ctx *app.Context, route plugin.Route, next http.HandlerFunc) http.HandlerFunc {
	if route.Method == http.MethodGet || route.Method == http.MethodHead || route.Method == http.MethodOptions {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)
		recorder := &auditResponseWriter{ResponseWriter: w}
		next(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		userID, _ := currentUserID(r.Context())
		username, _ := r.Context().Value(ctxkey.Username).(string)
		module, targetType := auditResource(route.Path)
		errorCode, errorMessage := auditError(recorder.body)
		input := store.OperationLogInput{
			UserID: userID, Username: username, Module: module, Action: auditAction(route.Method),
			Permission: auditAttribution(route), Method: route.Method, Path: r.URL.Path,
			TargetType: targetType, TargetID: pathvarValue(r, "id"), RequestID: requestID,
			IP: clientIP(r), UserAgent: r.UserAgent(), Success: status < http.StatusBadRequest,
			StatusCode: status, DurationMS: time.Since(startedAt).Milliseconds(),
			ErrorCode: errorCode, ErrorMessage: errorMessage,
		}
		if err := ctx.Auth.InsertOperationLog(r.Context(), input); err != nil {
			logx.Errorw("insert operation log failed",
				logx.Field("requestId", requestID),
				logx.Field("path", r.URL.Path),
				logx.Field("error", err.Error()),
			)
		}
		if status >= http.StatusInternalServerError {
			if err := ctx.Auth.InsertErrorLog(r.Context(), store.ErrorLogInput{
				RequestID: requestID, UserID: userID, Module: module, Method: route.Method,
				Path: r.URL.Path, ErrorCode: errorCode, Message: errorMessage, IP: clientIP(r),
			}); err != nil {
				logx.Errorw("insert error log failed",
					logx.Field("requestId", requestID),
					logx.Field("path", r.URL.Path),
					logx.Field("error", err.Error()),
				)
			}
		}
	}
}

// auditAttribution resolves the permission code recorded in audit rows:
// explicit AuditPermission first, then Permission, then AnyOf[0].
func auditAttribution(route plugin.Route) string {
	if route.AuditPermission != "" {
		return route.AuditPermission
	}
	if route.Permission != "" {
		return route.Permission
	}
	if len(route.AnyOf) > 0 {
		return route.AnyOf[0]
	}
	return ""
}

func auditResource(routePath string) (string, string) {
	parts := strings.Split(strings.Trim(routePath, "/"), "/")
	if len(parts) >= 3 {
		return parts[2], parts[2]
	}
	return "system", ""
}

func auditAction(method string) string {
	switch method {
	case http.MethodPost:
		return "create_or_execute"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	default:
		return strings.ToLower(method)
	}
}

func auditError(body []byte) (string, string) {
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	if payload.Code == 0 {
		return "", ""
	}
	return strconv.Itoa(payload.Code), payload.Message
}
