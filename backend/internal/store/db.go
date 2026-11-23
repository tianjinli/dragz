package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"dragz-admin/backend/internal/pkg/ctxkey"
)

// DB wraps sql.DB for the PostgreSQL store implementation.
type DB struct {
	*sql.DB
}

// Tx is a transaction handle. It exists so repositories can accept a *Tx instead of a bare *sql.Tx.
// A nested Tx shares the parent connection and commits or rolls back a savepoint.
type Tx struct {
	*sql.Tx
	nested    bool
	savepoint string
	done      atomic.Bool
}

var savepointSeq atomic.Uint64

func nextSavepoint() string {
	return "sp_" + strconv.FormatUint(savepointSeq.Add(1), 10)
}

// OpenPostgres is shorthand for Open with the postgres driver.
func OpenPostgres(dataSource string) (*DB, error) {
	return Open("postgres", dataSource)
}

// Open opens a database by driver name. Currently only postgres/postgresql are supported.
// Additional drivers should be added here when implementing multi-db store packages.
func Open(driver, dataSource string) (*DB, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "postgres", "postgresql":
		db, err := sql.Open("pgx", dataSource)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		db.SetConnMaxLifetime(3 * time.Minute)
		db.SetMaxOpenConns(20)
		db.SetMaxIdleConns(10)
		if err := db.Ping(); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		}
		return &DB{DB: db}, nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", driver)
	}
}

// RoleBypassesRLS reports whether the connected role is a superuser or has
// BYPASSRLS. Production refuses such an application identity: PostgreSQL does
// not apply row-level security to either attribute, even FORCE RLS.
func RoleBypassesRLS(ctx context.Context, db *DB) (bool, string, error) {
	if db == nil {
		return false, "", sql.ErrConnDone
	}
	var role string
	var bypasses bool
	err := db.QueryRowContext(ctx, `
SELECT rolname, (rolsuper OR rolbypassrls)
FROM pg_roles
WHERE rolname = current_user`).Scan(&role, &bypasses)
	return bypasses, role, err
}

// BeginTx starts a transaction and applies the tenant session variables from
// ctx. Both are SET LOCAL (set_config's third argument is true), so they die
// with the transaction and cannot leak onto a pooled connection.
// The caller must Commit or Rollback the returned handle.
//
// When ctx already carries a transaction, BeginTx does not take another
// connection. It opens a savepoint on that transaction: Commit releases it
// and Rollback undoes it. A deferred Rollback after a successful nested
// Commit returns sql.ErrTxDone, matching database/sql.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	if parent, ok := ctxkey.TxFrom(ctx); ok {
		name := nextSavepoint()
		if _, err := parent.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
			return nil, err
		}
		return &Tx{Tx: parent, nested: true, savepoint: name}, nil
	}
	tx, err := beginTenantTx(ctx, db.DB, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx}, nil
}

// Commit finishes the transaction, or releases the nested savepoint.
func (tx *Tx) Commit() error {
	if tx == nil || tx.Tx == nil {
		return sql.ErrTxDone
	}
	if !tx.nested {
		return tx.Tx.Commit()
	}
	if !tx.done.CompareAndSwap(false, true) {
		return sql.ErrTxDone
	}
	if _, err := tx.Tx.ExecContext(context.Background(), "RELEASE SAVEPOINT "+tx.savepoint); err != nil {
		_, _ = tx.Tx.ExecContext(context.Background(), "ROLLBACK TO SAVEPOINT "+tx.savepoint)
		_, _ = tx.Tx.ExecContext(context.Background(), "RELEASE SAVEPOINT "+tx.savepoint)
		return err
	}
	return nil
}

// Rollback aborts the transaction, or undoes the nested savepoint.
func (tx *Tx) Rollback() error {
	if tx == nil || tx.Tx == nil {
		return sql.ErrTxDone
	}
	if !tx.nested {
		return tx.Tx.Rollback()
	}
	if !tx.done.CompareAndSwap(false, true) {
		return sql.ErrTxDone
	}
	if _, err := tx.Tx.ExecContext(context.Background(), "ROLLBACK TO SAVEPOINT "+tx.savepoint); err != nil {
		return err
	}
	_, err := tx.Tx.ExecContext(context.Background(), "RELEASE SAVEPOINT "+tx.savepoint)
	return err
}

// beginTenantTx opens a transaction and installs the tenant session variables.
func beginTenantTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := setTenantSession(ctx, tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// setTenantSession writes app.tenant_id and app.rls_bypass for this transaction.
func setTenantSession(ctx context.Context, tx *sql.Tx) error {
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	bypass := "false"
	if ctxkey.RLSBypassFrom(ctx) {
		bypass = "true"
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.tenant_id', $1, true)`, strconv.FormatUint(tenantID, 10)); err != nil {
		return fmt.Errorf("set app.tenant_id: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.rls_bypass', $1, true)`, bypass); err != nil {
		return fmt.Errorf("set app.rls_bypass: %w", err)
	}
	return nil
}
