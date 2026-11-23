package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// DBTX must not invent a transaction. Without one it returns the plain pool, so
// a tenant query fails closed under FORCE RLS instead of silently succeeding
// with no app.tenant_id set.
func TestDBTXReturnsPlainPoolWithoutTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectQuery(`SELECT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(2))

	storeDB := &DB{DB: db}
	got := DBTX(ctxkey.WithTenantID(context.Background(), 7), storeDB)
	if _, ok := got.(ctxDBTX); !ok {
		t.Fatalf("DBTX without ctx transaction = %T, want ctxDBTX", got)
	}

	var n int
	if err := got.QueryRowContext(context.Background(), "SELECT 1").Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 2 {
		t.Fatalf("n = %d, want 2", n)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDBTXUsesContextTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock)
	mock.ExpectQuery(`SELECT 1`).
		WillReturnRows(sqlmock.NewRows([]string{"n"}).AddRow(3))
	mock.ExpectCommit()

	storeDB := &DB{DB: db}
	tx, err := storeDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	ctx := ctxkey.WithTx(ctxkey.WithTenantID(context.Background(), 3), tx.Tx)

	got := DBTX(ctx, storeDB)
	if got != sqlcstore.DBTX(tx.Tx) {
		t.Fatalf("DBTX with ctx transaction = %T, want the bound transaction", got)
	}

	var n int
	if err := got.QueryRowContext(ctx, "SELECT 1").Scan(&n); err != nil {
		t.Fatalf("query: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestDBTXNilDBReturnsNil(t *testing.T) {
	if got := DBTX(context.Background(), (*DB)(nil)); got != nil {
		t.Fatalf("DBTX(nil) = %T, want nil", got)
	}
}

func TestQueriesBindsContextTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock)
	mock.ExpectCommit()

	storeDB := &DB{DB: db}
	tx, err := storeDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	ctx := ctxkey.WithTx(context.Background(), tx.Tx)
	if Queries(ctx, storeDB) == nil {
		t.Fatal("Queries returned nil")
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestNestedBeginTxCommitsSavepoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock)
	mock.ExpectExec(`SAVEPOINT sp_\d+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`RELEASE SAVEPOINT sp_\d+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	storeDB := &DB{DB: db}
	outer, err := storeDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	ctx := ctxkey.WithTx(context.Background(), outer.Tx)
	inner, err := storeDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("nested begin: %v", err)
	}
	defer func() { _ = inner.Rollback() }()
	if err := inner.Commit(); err != nil {
		t.Fatalf("nested commit: %v", err)
	}
	if err := outer.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestNestedBeginTxRollsBackToSavepoint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectBegin()
	expectTenantSession(mock)
	mock.ExpectExec(`SAVEPOINT sp_\d+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ROLLBACK TO SAVEPOINT sp_\d+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`RELEASE SAVEPOINT sp_\d+`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	storeDB := &DB{DB: db}
	outer, err := storeDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	ctx := ctxkey.WithTx(context.Background(), outer.Tx)
	inner, err := storeDB.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("nested begin: %v", err)
	}
	if err := inner.Rollback(); err != nil {
		t.Fatalf("nested rollback: %v", err)
	}
	if err := inner.Rollback(); err != sql.ErrTxDone {
		t.Fatalf("second rollback = %v, want ErrTxDone", err)
	}
	if err := outer.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	mock.ExpectClose()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
