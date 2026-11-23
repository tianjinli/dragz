package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/store"
)

func TestIsPlatformHost(t *testing.T) {
	ctx := &app.Context{Config: app.ConfigView{PlatformHosts: []string{"api.saas.local"}}}
	if !isPlatformHost(ctx, "10.0.0.8:5280") {
		t.Fatal("raw IP must be a platform host")
	}
	if !isPlatformHost(ctx, "localhost:5280") {
		t.Fatal("localhost must be a platform host")
	}
	if !isPlatformHost(ctx, "API.saas.local") {
		t.Fatal("configured platform host must match")
	}
	if isPlatformHost(ctx, "acme.localhost") {
		t.Fatal("slug host must not be a platform host")
	}
}

func TestResolveTenantHostMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock, "4", "true")
	mock.ExpectQuery(`FROM sys_tenant_member`).
		WithArgs(uint64(7), uint64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "is_platform_operator"}).
			AddRow(int64(4), "other", "Other", false))
	mock.ExpectCommit()

	appCtx := &app.Context{
		DB:      &store.DB{DB: db},
		Tenants: store.NewTenantRepository(&store.DB{DB: db}),
	}
	handler := resolveTenant(appCtx, func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not run when the host tenant disagrees")
	})
	req := withClaimedTenant(httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil), 7, "admin", 4)
	req = req.WithContext(ctxkey.WithHostTenantID(req.Context(), 9))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "errors.tenant_host_mismatch") {
		t.Fatalf("body = %s, want host mismatch", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestIssueTenantIDRequiresHostMembership(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock, "0", "true")
	mock.ExpectQuery(`FROM sys_tenant_member`).
		WithArgs(uint64(7), uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "is_platform_operator"}))
	mock.ExpectRollback()

	appCtx := &app.Context{Tenants: store.NewTenantRepository(&store.DB{DB: db})}
	handler := func(w http.ResponseWriter, r *http.Request) {
		id, issueErr := issueTenantID(r.Context(), appCtx, 7)
		if failIssuedTenant(w, r, issueErr) {
			return
		}
		ok(w, r, map[string]uint64{"tenantId": id})
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req = req.WithContext(ctxkey.WithHostTenantID(req.Context(), 9))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "errors.tenant_forbidden") {
		t.Fatalf("body = %s, want tenant forbidden", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestIssueTenantIDUsesHostTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock, "0", "true")
	mock.ExpectQuery(`FROM sys_tenant_member`).
		WithArgs(uint64(7), uint64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "is_platform_operator"}).
			AddRow(int64(9), "acme", "Acme", false))
	mock.ExpectCommit()

	appCtx := &app.Context{Tenants: store.NewTenantRepository(&store.DB{DB: db})}
	handler := func(w http.ResponseWriter, r *http.Request) {
		id, issueErr := issueTenantID(r.Context(), appCtx, 7)
		if failIssuedTenant(w, r, issueErr) {
			return
		}
		ok(w, r, map[string]uint64{"tenantId": id})
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req = req.WithContext(ctxkey.WithHostTenantID(req.Context(), 9))
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"tenantId":9`) {
		t.Fatalf("body = %s, want tenant 9", rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
