package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dragz-admin/backend/internal/app"
)

func TestWSHandlerRequiresToken(t *testing.T) {
	handler := wsHandler(&app.Context{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	handler(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
