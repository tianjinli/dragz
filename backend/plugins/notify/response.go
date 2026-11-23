package notify

import (
	"encoding/json"
	"net/http"

	"github.com/zeromicro/go-zero/rest/pathvar"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
)

// A plugin owns its response helpers: it must not import internal/httpapi.
// The envelope is byte-for-byte the host one so the admin UI can decode it.
type envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, envelope{Code: apierrors.CodeOK, Message: "ok", Data: data})
}

func fail(w http.ResponseWriter, status, code int, key string, ctx *app.Context, r *http.Request) {
	msg := key
	if ctx != nil && ctx.Translator != nil {
		msg = ctx.Translator.T(r, key)
	}
	writeJSON(w, status, envelope{Code: code, Message: msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeJSON(w, http.StatusBadRequest, envelope{Code: apierrors.CodeValidation, Message: "invalid request body"})
		return false
	}
	return true
}

func queryInt(raw string, def int) int {
	if raw == "" {
		return def
	}
	n := 0
	for _, c := range raw {
		if c < '0' || c > '9' {
			return def
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return def
	}
	return n
}

func pathID(r *http.Request) (uint64, bool) {
	raw := pathvar.Vars(r)["id"]
	n := uint64(0)
	if raw == "" {
		return 0, false
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + uint64(c-'0')
	}
	return n, n > 0
}
