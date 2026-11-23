// Package tenantconn reserves the dedicated-database exit for large tenants.
// Day-1 traffic stays on the shared pool; only a compliance or premium
// isolation purchase should install a non-default Resolver.
package tenantconn

import (
	"context"
	"database/sql"
)

// Resolver picks the database handle for one tenant. The default implementation
// always returns the shared pool; a future adapter can route selected tenants
// to a dedicated DSN without changing business repositories.
type Resolver interface {
	DB(ctx context.Context, tenantID uint64) (*sql.DB, error)
}

// Shared always returns the same pool.
type Shared struct {
	Pool *sql.DB
}

// DB implements Resolver.
func (s Shared) DB(_ context.Context, _ uint64) (*sql.DB, error) {
	if s.Pool == nil {
		return nil, sql.ErrConnDone
	}
	return s.Pool, nil
}
