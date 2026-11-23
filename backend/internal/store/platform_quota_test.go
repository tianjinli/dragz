package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSyncQuotasFromPlanSeedsSeatsAndSkipsUnparsedValues(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs("0").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs("true").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`entitlement_key LIKE 'quota\.%'`).
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"entitlement_key", "entitlement_value"}).
			AddRow("quota.seats", "100").
			AddRow("quota.storage", "nope"))
	mock.ExpectExec(`INSERT INTO sys_tenant_quota`).
		WithArgs(uint64(9), "seats", uint64(100)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewTenantRepository(&DB{DB: db})
	if err := repo.SyncQuotasFromPlan(context.Background(), 9, 1); err != nil {
		t.Fatalf("SyncQuotasFromPlan: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestSyncQuotasFromPlanSkipsZeroIDs(t *testing.T) {
	repo := NewTenantRepository(&DB{})
	if err := repo.SyncQuotasFromPlan(context.Background(), 0, 1); err != nil {
		t.Fatalf("tenant 0: %v", err)
	}
	if err := repo.SyncQuotasFromPlan(context.Background(), 4, 0); err != nil {
		t.Fatalf("plan 0: %v", err)
	}
}
