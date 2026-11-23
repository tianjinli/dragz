package store

import (
	"context"
	"database/sql"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// ctxDBTX routes every generated query through the transaction stored on ctx,
// falling back to the connection pool when the caller has none.
//
// This matters because repositories bind sqlc once at construction time
// (sqlcstore.New(newSessionDBTX(db))). The context, not the constructor, decides
// which connection a call runs on: a request handler that opened a transaction
// and stored it with ctxkey.WithTx gets that transaction, so the session
// variables set by BeginTx (app.tenant_id / app.rls_bypass) apply to the query.
//
// It deliberately does NOT open a transaction on its own. Implicit wrapping
// would require committing at an unknown moment, and the previous attempt
// patched database/sql internals through unsafe to guess that moment. Without a
// transaction the query runs with no tenant session, which under FORCE ROW
// LEVEL SECURITY fails closed rather than leaking another tenant's rows.
type ctxDBTX struct {
	pool *sql.DB
}

func (c ctxDBTX) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	if tx, ok := ctxkey.TxFrom(ctx); ok {
		return tx.ExecContext(ctx, query, args...)
	}
	return c.pool.ExecContext(ctx, query, args...)
}

func (c ctxDBTX) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if tx, ok := ctxkey.TxFrom(ctx); ok {
		return tx.PrepareContext(ctx, query)
	}
	return c.pool.PrepareContext(ctx, query)
}

func (c ctxDBTX) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	if tx, ok := ctxkey.TxFrom(ctx); ok {
		return tx.QueryContext(ctx, query, args...)
	}
	return c.pool.QueryContext(ctx, query, args...)
}

func (c ctxDBTX) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	if tx, ok := ctxkey.TxFrom(ctx); ok {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return c.pool.QueryRowContext(ctx, query, args...)
}

// DBTX binds generated queries to the transaction already stored on ctx.
//
// Tenant code MUST run inside a request/job transaction: that is where
// set_config('app.tenant_id', ..., true) lives. Falling back to the pool without
// a tenant is deliberately not papered over here. With FORCE ROW LEVEL SECURITY
// and a non-superuser application role, such a query sees no tenant rows or
// fails, which is the safe outcome.
func DBTX(ctx context.Context, fallback sqlcstore.DBTX) sqlcstore.DBTX {
	if fallback == nil {
		return nil
	}
	if tx, ok := ctxkey.TxFrom(ctx); ok {
		return tx
	}
	switch db := fallback.(type) {
	case *DB:
		if db == nil {
			return nil
		}
		return ctxDBTX{pool: db.DB}
	case *sql.DB:
		return ctxDBTX{pool: db}
	default:
		return fallback
	}
}

// Queries binds sqlc to the transaction on ctx when one is present.
func Queries(ctx context.Context, fallback sqlcstore.DBTX) *sqlcstore.Queries {
	return sqlcstore.New(DBTX(ctx, fallback))
}

// newSessionDBTX is the handle repository constructors bind once. It resolves
// the connection per call from ctx, so a request-scoped transaction is honoured
// without re-creating the repository.
func newSessionDBTX(db *DB) sqlcstore.DBTX {
	if db == nil {
		return nil
	}
	return ctxDBTX{pool: db.DB}
}
