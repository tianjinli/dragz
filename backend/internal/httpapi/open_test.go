package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

func TestRequireOpenScope(t *testing.T) {
	next := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	handler := requireOpenScope([]string{openScopeMe}, next)

	tests := []struct {
		name   string
		set    bool
		scopes []string
		status int
		key    string
	}{
		{name: "jwt", status: http.StatusUnauthorized, key: "errors.open_token_required"},
		{name: "empty", set: true, scopes: []string{}, status: http.StatusUnauthorized, key: "errors.open_token_required"},
		{name: "other scope", set: true, scopes: []string{openScopeTenant}, status: http.StatusForbidden, key: "errors.token_scope"},
		{name: "exact", set: true, scopes: []string{openScopeMe}, status: http.StatusNoContent},
		{name: "blanket", set: true, scopes: []string{openScopeAll}, status: http.StatusNoContent},
		{name: "star", set: true, scopes: []string{"*"}, status: http.StatusNoContent},
		{name: "among others", set: true, scopes: []string{"system:user:query", openScopeMe}, status: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/open/v1/me", nil)
			if tt.set {
				request = withTokenScopes(request, tt.scopes)
			}
			handler(recorder, request)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, tt.status, recorder.Body.String())
			}
			if tt.key != "" && !strings.Contains(recorder.Body.String(), tt.key) {
				t.Fatalf("body = %s, want key %s", recorder.Body.String(), tt.key)
			}
		})
	}
}

func TestGuardOpenRouteRejectsMissingToken(t *testing.T) {
	handler := guard(&app.Context{}, plugin.Route{OpenScopes: []string{openScopeMe}}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not run")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/me", nil)
	request = withUserContext(request, 7, "ada")
	handler(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "errors.open_token_required") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestAuthenticateRejectsJWTOnOpenAPI(t *testing.T) {
	tokens := security.NewTokenManager("test-secret", "dragz", 3600, 7200)
	pair, err := tokens.Issue(42, "admin", 0, 3)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	handler := authenticate(&app.Context{Tokens: tokens}, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("jwt reached the open handler")
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	handler(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "errors.open_token_required") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestOpenMeReturnsTokenPrincipal(t *testing.T) {
	appCtx := &app.Context{}
	handler := guard(appCtx, plugin.Route{OpenScopes: []string{openScopeMe}}, openMeHandler(appCtx))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/me", nil)
	request = withUserContext(request, 7, "ada")
	request = withTokenScopes(request, []string{openScopeMe, "system:user:query"})
	request = request.WithContext(ctxkey.WithTenantID(request.Context(), 3))
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Code int            `json:"code"`
		Data openMeResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if body.Code != 0 || body.Data.UserID != 7 || body.Data.Username != "ada" || body.Data.TenantID != 3 {
		t.Fatalf("body = %+v", body)
	}
	if len(body.Data.Scopes) != 2 || body.Data.Scopes[0] != openScopeMe {
		t.Fatalf("scopes = %#v", body.Data.Scopes)
	}
}

func TestOpenPingReturnsTenant(t *testing.T) {
	handler := guard(&app.Context{}, plugin.Route{OpenScopes: []string{openScopePing}}, openPingHandler(nil))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/ping", nil)
	request = withTokenScopes(request, []string{openScopeAll})
	request = request.WithContext(ctxkey.WithTenantID(request.Context(), 9))
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data openPingResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Data.OK || body.Data.TenantID != 9 {
		t.Fatalf("data = %+v", body.Data)
	}
}

func TestOpenTenantReadsCurrentTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	mock.ExpectQuery(`SELECT id, code, name, status\s+FROM sys_tenant\s+WHERE id = \$1`).
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "status"}).
			AddRow(int64(9), "acme", "Acme", int64(1)))

	appCtx := &app.Context{Tenants: store.NewTenantRepository(&store.DB{DB: db})}
	handler := openTenantHandler(appCtx)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/tenant", nil)
	request = request.WithContext(ctxkey.WithTx(ctxkey.WithTenantID(request.Context(), 9), tx))
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", recorder.Code, recorder.Body.String())
	}
	var body struct {
		Data openTenantResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v body=%s", err, recorder.Body.String())
	}
	if body.Data.ID != 9 || body.Data.Code != "acme" || body.Data.Name != "Acme" || body.Data.Status != 1 {
		t.Fatalf("data = %+v", body.Data)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestOpenTenantMissing(t *testing.T) {
	handler := openTenantHandler(&app.Context{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/tenant", nil)
	handler(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "errors.tenant_not_found") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

func TestOpenTenantNotFoundRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	mock.ExpectQuery(`SELECT id, code, name, status`).
		WithArgs(int64(4)).
		WillReturnError(sql.ErrNoRows)

	appCtx := &app.Context{Tenants: store.NewTenantRepository(&store.DB{DB: db})}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/open/v1/tenant", nil)
	request = request.WithContext(ctxkey.WithTx(ctxkey.WithTenantID(request.Context(), 4), tx))
	openTenantHandler(appCtx)(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 body=%s", recorder.Code, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestScopesAllowedAcceptsOpenLiterals(t *testing.T) {
	if !scopesAllowed([]string{openScopeMe}, nil, false) {
		t.Fatal("open.me should be grantable without an RBAC permission")
	}
	if !scopesAllowed([]string{openScopeAll, openScopeTenant, openScopePing}, nil, false) {
		t.Fatal("open literals should be grantable together")
	}
	if scopesAllowed([]string{"open.other"}, nil, false) {
		t.Fatal("unknown scope should be rejected")
	}
	if scopesAllowed([]string{"*"}, nil, false) {
		t.Fatal("star stays limited to super administrators")
	}
	if !scopesAllowed([]string{"*"}, nil, true) {
		t.Fatal("super administrator may grant star")
	}
}
