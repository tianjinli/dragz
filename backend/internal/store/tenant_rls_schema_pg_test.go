package store

import (
	"context"
	"os"
	"testing"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
)

// TestTenantRLSIsolatesRealTables is the acceptance test the schema exists for.
//
// The predicate test above proves that PostgreSQL applies a policy at all. This
// one proves that the shipped tables are actually protected: it seeds two
// tenants through the platform role, then re-reads them through the application
// role and requires that each session sees exactly its own tenant's rows.
//
// Skipped unless both DRAGZ_VERIFY_DSN (application role, no BYPASSRLS) and
// DRAGZ_VERIFY_PLATFORM_DSN (platform role, BYPASSRLS) are set and point at a
// migrated, disposable database. `deploy/verify-rls.sh` builds exactly that.
func TestTenantRLSIsolatesRealTables(t *testing.T) {
	appDSN := os.Getenv("DRAGZ_VERIFY_DSN")
	platformDSN := os.Getenv("DRAGZ_VERIFY_PLATFORM_DSN")
	if appDSN == "" || platformDSN == "" {
		t.Skip("DRAGZ_VERIFY_DSN and DRAGZ_VERIFY_PLATFORM_DSN not set; skipping the real-table isolation test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	appDB, err := OpenPostgres(appDSN)
	if err != nil {
		t.Fatalf("open application role: %v", err)
	}
	t.Cleanup(func() { _ = appDB.Close() })

	platformDB, err := OpenPostgres(platformDSN)
	if err != nil {
		t.Fatalf("open platform role: %v", err)
	}
	t.Cleanup(func() { _ = platformDB.Close() })

	if bypasses, role, err := RoleBypassesRLS(ctx, appDB); err != nil {
		t.Fatalf("application role attributes: %v", err)
	} else if bypasses {
		t.Fatalf("application role %q is a superuser or has BYPASSRLS; this test cannot observe isolation", role)
	}
	if bypasses, role, err := RoleBypassesRLS(ctx, platformDB); err != nil {
		t.Fatalf("platform role attributes: %v", err)
	} else if !bypasses {
		t.Fatalf("platform role %q does not bypass row level security; seeding would be filtered", role)
	}

	const (
		tenantA = uint64(990901)
		tenantB = uint64(990902)
	)
	userA, userB := seedTenantFixtures(t, ctx, platformDB, tenantA, tenantB)
	t.Cleanup(func() {
		cleanupTenantFixtures(context.Background(), platformDB, tenantA, tenantB, userA, userB)
	})

	// Tenant A must see its own member and notice and nothing from tenant B.
	seenA := readTenantRows(t, ctx, appDB, tenantA)
	if seenA.members != 1 || seenA.notices != 1 {
		t.Fatalf("tenant %d saw members=%d notices=%d, want 1 and 1", tenantA, seenA.members, seenA.notices)
	}
	if seenA.sawUserB {
		t.Fatalf("tenant %d could read tenant %d's membership row", tenantA, tenantB)
	}

	seenB := readTenantRows(t, ctx, appDB, tenantB)
	if seenB.members != 1 || seenB.notices != 1 {
		t.Fatalf("tenant %d saw members=%d notices=%d, want 1 and 1", tenantB, seenB.members, seenB.notices)
	}
	if seenB.sawUserA {
		t.Fatalf("tenant %d could read tenant %d's membership row", tenantB, tenantA)
	}

	// A session with no tenant must read nothing: this is the fail-closed case
	// that a missing transaction would otherwise hide.
	empty := readTenantRows(t, ctx, appDB, 0)
	if empty.members != 0 || empty.notices != 0 {
		t.Fatalf("session without a tenant saw members=%d notices=%d, want 0 and 0", empty.members, empty.notices)
	}
}

type tenantRowCounts struct {
	members  int
	notices  int
	sawUserA bool
	sawUserB bool
}

func readTenantRows(t *testing.T, ctx context.Context, db *DB, tenantID uint64) tenantRowCounts {
	t.Helper()
	// A real request transaction, not an ad-hoc query: this is where
	// set_config('app.tenant_id', ...) lives.
	tx, err := db.BeginTx(ctxkey.WithTenantID(ctx, tenantID), nil)
	if err != nil {
		t.Fatalf("begin tenant %d: %v", tenantID, err)
	}
	defer func() { _ = tx.Rollback() }()

	var counts tenantRowCounts
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sys_tenant_member`).Scan(&counts.members); err != nil {
		t.Fatalf("count members for tenant %d: %v", tenantID, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sys_notice WHERE deleted_at IS NULL`).Scan(&counts.notices); err != nil {
		t.Fatalf("count notices for tenant %d: %v", tenantID, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sys_tenant_member WHERE user_id = $1)`, uint64(990901001)).Scan(&counts.sawUserA); err != nil {
		t.Fatalf("probe user A for tenant %d: %v", tenantID, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sys_tenant_member WHERE user_id = $1)`, uint64(990902001)).Scan(&counts.sawUserB); err != nil {
		t.Fatalf("probe user B for tenant %d: %v", tenantID, err)
	}
	return counts
}

func seedTenantFixtures(t *testing.T, ctx context.Context, db *DB, tenantA, tenantB uint64) (uint64, uint64) {
	t.Helper()
	tx, err := db.BeginTx(ctxkey.WithRLSBypass(ctx, true), nil)
	if err != nil {
		t.Fatalf("begin seed: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	userA, userB := uint64(990901001), uint64(990902001)
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO sys_tenant (id, code, name, status) VALUES ($1, $2, $3, 1) ON CONFLICT (id) DO NOTHING`,
			[]any{tenantA, "rls-probe-a", "RLS probe A"}},
		{`INSERT INTO sys_tenant (id, code, name, status) VALUES ($1, $2, $3, 1) ON CONFLICT (id) DO NOTHING`,
			[]any{tenantB, "rls-probe-b", "RLS probe B"}},
		{`INSERT INTO sys_user (id, username, password_hash, display_name, status) VALUES ($1, $2, 'x', 'probe a', 1)
		  ON CONFLICT (id) DO NOTHING`, []any{userA, "rls-probe-a"}},
		{`INSERT INTO sys_user (id, username, password_hash, display_name, status) VALUES ($1, $2, 'x', 'probe b', 1)
		  ON CONFLICT (id) DO NOTHING`, []any{userB, "rls-probe-b"}},
		{`INSERT INTO sys_tenant_member (tenant_id, user_id, status) VALUES ($1, $2, 1)
		  ON CONFLICT (tenant_id, user_id) DO NOTHING`, []any{tenantA, userA}},
		{`INSERT INTO sys_tenant_member (tenant_id, user_id, status) VALUES ($1, $2, 1)
		  ON CONFLICT (tenant_id, user_id) DO NOTHING`, []any{tenantB, userB}},
		{`INSERT INTO sys_notice (tenant_id, title, content, notice_type, status, created_by)
		  VALUES ($1, 'probe a', 'body', 'notice', 'draft', $2)`, []any{tenantA, userA}},
		{`INSERT INTO sys_notice (tenant_id, title, content, notice_type, status, created_by)
		  VALUES ($1, 'probe b', 'body', 'notice', 'draft', $2)`, []any{tenantB, userB}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed (%s): %v", statement.sql, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit seed: %v", err)
	}
	return userA, userB
}

func cleanupTenantFixtures(ctx context.Context, db *DB, tenantA, tenantB, userA, userB uint64) {
	tx, err := db.BeginTx(ctxkey.WithRLSBypass(ctx, true), nil)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM sys_notice WHERE tenant_id = ANY($1)`, []any{[]uint64{tenantA, tenantB}}},
		{`DELETE FROM sys_tenant_member WHERE tenant_id = ANY($1)`, []any{[]uint64{tenantA, tenantB}}},
		{`DELETE FROM sys_user WHERE id = ANY($1)`, []any{[]uint64{userA, userB}}},
		{`DELETE FROM sys_tenant WHERE id = ANY($1)`, []any{[]uint64{tenantA, tenantB}}},
	} {
		if _, err := tx.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return
		}
	}
	_ = tx.Commit()
}
