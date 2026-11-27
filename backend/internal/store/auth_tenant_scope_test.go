package store

import (
	"context"
	"testing"

	"dragz-admin/backend/internal/pkg/ctxkey"

	"github.com/DATA-DOG/go-sqlmock"
)

const tenantGrantFilter = `r\.tenant_id = \$2 OR ur\.tenant_id = \$2`

func TestPermissionCodesMenusAndRolesUseTenantFromContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectQuery(tenantGrantFilter).
		WithArgs(uint64(3), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}).AddRow("user.create"))

	mock.ExpectQuery(tenantGrantFilter).
		WithArgs(uint64(3), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"code", "name"}))

	mock.ExpectQuery(tenantGrantFilter).
		WithArgs(uint64(3), uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "parent_id", "type", "name", "code", "route_name", "route_path",
			"component", "redirect", "icon", "sort",
		}))

	repo := NewAuthRepository(&DB{DB: db})
	ctx := ctxkey.WithTenantID(context.Background(), 7)
	codes, err := repo.PermissionCodes(ctx, 3, false)
	if err != nil {
		t.Fatalf("PermissionCodes: %v", err)
	}
	if len(codes) != 1 || codes[0] != "user.create" {
		t.Fatalf("codes = %#v", codes)
	}
	if _, err := repo.UserRoles(ctx, 3); err != nil {
		t.Fatalf("UserRoles: %v", err)
	}
	if _, err := repo.Menus(ctx, 3, false); err != nil {
		t.Fatalf("Menus: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPermissionCodesUsesZeroTenantWhenContextHasNone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)
	mock.ExpectQuery(tenantGrantFilter).
		WithArgs(uint64(3), uint64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"code"}))

	repo := NewAuthRepository(&DB{DB: db})
	codes, err := repo.PermissionCodes(context.Background(), 3, false)
	if err != nil {
		t.Fatalf("PermissionCodes: %v", err)
	}
	if len(codes) != 0 {
		t.Fatalf("codes = %#v", codes)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestCreateAPITokenStoresTenantFromContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectQuery(`INSERT INTO sys_api_token`).
		WithArgs(uint64(1), uint64(9), "ci", "prefix", "hash", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(11)))

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).
		WithArgs("8").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).
		WithArgs("false").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`INSERT INTO sys_api_token`).
		WithArgs(uint64(8), uint64(9), "ci", "prefix", "hash", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(12)))
	mock.ExpectCommit()

	repo := NewAuthRepository(&DB{DB: db})
	id, err := repo.CreateAPIToken(context.Background(), 9, "ci", "prefix", "hash", []string{"read"}, nil, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken default tenant: %v", err)
	}
	if id != 11 {
		t.Fatalf("id = %d", id)
	}

	// A request transaction on ctx is honoured even though the repository bound
	// its sqlc handle at construction time.
	storeDB := &DB{DB: db}
	tenantCtx := ctxkey.WithTenantID(context.Background(), 8)
	tx, err := storeDB.BeginTx(tenantCtx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	tenantCtx = ctxkey.WithTx(tenantCtx, tx.Tx)
	id, err = repo.CreateAPIToken(tenantCtx, 9, "ci", "prefix", "hash", []string{"read"}, nil, nil)
	if err != nil {
		t.Fatalf("CreateAPIToken tenant 8: %v", err)
	}
	if id != 12 {
		t.Fatalf("id = %d", id)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
