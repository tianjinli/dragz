package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/store"
)

func expectTenantSession(mock sqlmock.Sqlmock, tenantID, bypass string) {
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs(tenantID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs(bypass).
		WillReturnResult(sqlmock.NewResult(0, 0))
}

func TestResolveTenantCommitsRequestTx(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock, "0", "true")
	mock.ExpectQuery(`FROM sys_tenant_member`).
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "is_platform_operator"}).
			AddRow(int64(4), "default", "Default", false))
	mock.ExpectCommit()

	mock.ExpectBegin()
	expectTenantSession(mock, "4", "false")
	mock.ExpectCommit()

	appCtx := &app.Context{
		DB:      &store.DB{DB: db},
		Tenants: store.NewTenantRepository(&store.DB{DB: db}),
	}
	var sawTenant uint64
	var sawTx bool
	handler := resolveTenant(appCtx, func(w http.ResponseWriter, r *http.Request) {
		sawTenant, _ = ctxkey.TenantIDFrom(r.Context())
		_, sawTx = ctxkey.TxFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	req := withClaimedTenant(httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil), 7, "admin", 0)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	if sawTenant != 4 || !sawTx {
		t.Fatalf("tenant = %d tx = %v, want tenant 4 and a transaction", sawTenant, sawTx)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestResolveTenantRollsBackOnServerError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock, "0", "true")
	mock.ExpectQuery(`FROM sys_tenant_member`).
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code", "name", "is_platform_operator"}).
			AddRow(int64(4), "default", "Default", false))
	mock.ExpectCommit()

	mock.ExpectBegin()
	expectTenantSession(mock, "4", "false")
	mock.ExpectRollback()

	appCtx := &app.Context{
		DB:      &store.DB{DB: db},
		Tenants: store.NewTenantRepository(&store.DB{DB: db}),
	}
	handler := resolveTenant(appCtx, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	req := withClaimedTenant(httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil), 7, "admin", 0)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
