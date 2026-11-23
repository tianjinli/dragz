package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestTenantRLSHidesOtherTenant checks the core isolation predicate: with
// app.tenant_id set to 1 a session cannot read tenant 2, and app.rls_bypass
// sees both.
//
// Skipped unless DRAGZ_VERIFY_DSN points at a disposable database. Also skipped
// when the role is a superuser or has BYPASSRLS: PostgreSQL never applies row
// level security to those roles, even with FORCE.
func TestTenantRLSHidesOtherTenant(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the tenant isolation test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var bypasses bool
	if err := db.QueryRowContext(ctx, `
SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&bypasses); err != nil {
		t.Fatalf("role attributes: %v", err)
	}
	if bypasses {
		t.Skip("current role bypasses row level security; the predicate cannot be observed")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range []string{
		`CREATE SCHEMA IF NOT EXISTS tenant_rls_probe`,
		`SET LOCAL search_path TO tenant_rls_probe`,
		`DROP TABLE IF EXISTS probe`,
		`CREATE FUNCTION app_current_tenant_id()
RETURNS BIGINT LANGUAGE sql STABLE AS $$
  SELECT CASE
    WHEN current_setting('app.tenant_id', true) IS NULL THEN NULL
    WHEN btrim(current_setting('app.tenant_id', true)) = '' THEN NULL
    ELSE btrim(current_setting('app.tenant_id', true))::BIGINT
  END
$$`,
		`CREATE FUNCTION app_rls_bypass()
RETURNS BOOLEAN LANGUAGE sql STABLE AS $$
  SELECT lower(COALESCE(current_setting('app.rls_bypass', true), '')) IN ('on', 'true', '1')
$$`,
		`CREATE TABLE probe (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  name TEXT NOT NULL
)`,
		`ALTER TABLE probe ENABLE ROW LEVEL SECURITY`,
		`ALTER TABLE probe FORCE ROW LEVEL SECURITY`,
		`CREATE POLICY tenant_isolation ON probe
  USING (app_rls_bypass() OR tenant_id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id())`,
		`SELECT set_config('app.rls_bypass', 'true', true)`,
		`INSERT INTO probe (tenant_id, name) VALUES (1, 'alpha'), (2, 'beta')`,
		`SELECT set_config('app.tenant_id', '1', true)`,
		`SELECT set_config('app.rls_bypass', 'false', true)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare (%s): %v", statement, err)
		}
	}

	seen, err := probeTenantIDs(ctx, tx)
	if err != nil {
		t.Fatalf("select as tenant 1: %v", err)
	}
	if len(seen) != 1 || seen[0] != 1 {
		t.Fatalf("tenant 1 saw %v, want [1]", seen)
	}

	if _, err := tx.ExecContext(ctx, `SELECT set_config('app.rls_bypass', 'true', true)`); err != nil {
		t.Fatalf("enable bypass: %v", err)
	}
	seen, err = probeTenantIDs(ctx, tx)
	if err != nil {
		t.Fatalf("select with bypass: %v", err)
	}
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("bypass saw %v, want [1 2]", seen)
	}
}

func probeTenantIDs(ctx context.Context, tx *Tx) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT tenant_id FROM probe ORDER BY tenant_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
