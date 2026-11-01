package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"

	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
)

type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func pathvarValue(r *http.Request, key string) string {
	return pathvar.Vars(r)[key]
}

func ok(w http.ResponseWriter, r *http.Request, data any) {
	httpx.WriteJsonCtx(r.Context(), w, http.StatusOK, envelope{Code: apierrors.CodeOK, Message: localized(r, "common.ok"), Data: data})
}

func fail(w http.ResponseWriter, r *http.Request, status, code int, message string, templateData ...map[string]any) {
	httpx.WriteJsonCtx(r.Context(), w, status, envelope{Code: code, Message: localized(r, message, templateData...)})
}

func failData(w http.ResponseWriter, r *http.Request, status, code int, message string, data any, templateData ...map[string]any) {
	httpx.WriteJsonCtx(r.Context(), w, status, envelope{Code: code, Message: localized(r, message, templateData...), Data: data})
}

func failHTTP(w http.ResponseWriter, r *http.Request, mapped apierrors.HTTPError) {
	data := mapped.Data
	if data != nil {
		if subjectKey, ok := data["SubjectKey"].(string); ok && subjectKey != "" {
			cloned := make(map[string]any, len(data)+1)
			for k, v := range data {
				if k == "SubjectKey" {
					continue
				}
				cloned[k] = v
			}
			cloned["Subject"] = localized(r, subjectKey)
			fail(w, r, mapped.Status, mapped.Code, mapped.Message, cloned)
			return
		}
	}
	fail(w, r, mapped.Status, mapped.Code, mapped.Message, data)
}

func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	if r.Body != nil && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(value); err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeBadRequest, "errors.bad_request")
			return false
		}
		return true
	}
	if err := httpx.ParseJsonBody(r, value); err != nil {
		fail(w, r, http.StatusBadRequest, apierrors.CodeBadRequest, "errors.bad_request")
		return false
	}
	return true
}

func currentUsername(ctx context.Context) string {
	value := ctx.Value(ctxkey.Username)
	username, _ := value.(string)
	return username
}

type updateStatusRequest struct {
	Status int8 `json:"status"`
}

func validStatus(value int8) bool {
	return value == 0 || value == 1
}
