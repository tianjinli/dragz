package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// DB wraps sql.DB for the PostgreSQL store implementation.
type DB struct {
	*sql.DB
}

// Tx is a transaction handle. It exists so repositories can accept a *Tx instead of a bare *sql.Tx.
type Tx struct {
	*sql.Tx
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

// BeginTx starts a transaction; the caller must Commit or Rollback it.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx}, nil
}
