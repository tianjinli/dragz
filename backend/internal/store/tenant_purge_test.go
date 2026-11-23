package store

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestTenantPurgeAllowedSkipsDefaultTenant(t *testing.T) {
	if TenantPurgeAllowed(DefaultTenantID) {
		t.Fatal("tenant 1 must never be purgeable")
	}
	if TenantPurgeAllowed(0) {
		t.Fatal("tenant 0 must never be purgeable")
	}
	if !TenantPurgeAllowed(2) {
		t.Fatal("tenant 2 should be purgeable")
	}
}

func TestScheduleTenantPurgeRejectsDefaultTenant(t *testing.T) {
	repo := NewTenantRepository(&DB{})
	err := repo.ScheduleTenantPurge(context.Background(), DefaultTenantID, 7)
	if !errors.Is(err, ErrProtected) {
		t.Fatalf("expected protected, got %v", err)
	}
}

func TestPurgeDueTenantsSkipsDefaultTenant(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs("0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs("true").
		WillReturnResult(sqlmock.NewResult(0, 0))
	// The query also excludes id 1. Returning that id here proves the Go
	// clamp still refuses to delete it.
	mock.ExpectQuery(`purge_status = 'pending_delete'`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))
	mock.ExpectCommit()
	mock.ExpectClose()

	repo := NewTenantRepository(&DB{DB: db})
	n, err := repo.PurgeDueTenants(context.Background())
	if err != nil {
		t.Fatalf("PurgeDueTenants: %v", err)
	}
	if n != 0 {
		t.Fatalf("purged %d tenants, want 0", n)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
