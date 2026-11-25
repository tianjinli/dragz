package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestTenantAdminPageCodesAllowlist(t *testing.T) {
	codes := tenantAdminPageCodes()
	want := map[string]bool{
		"dashboard": true, "system_member": true, "system_department": true,
		"system_position": true, "system_role": true, "system_billing": true,
		"system_config": true, "system_notice": true, "my_messages": true,
		"system_file": true, "system_file_recycle": true, "system_online": true,
		"system_token": true, "system_login_log": true, "system_operation_log": true,
	}
	if len(codes) != len(want) {
		t.Fatalf("allowlist length %d, want %d: %v", len(codes), len(want), codes)
	}
	for _, code := range codes {
		if !want[code] {
			t.Fatalf("unexpected allowlist code %q", code)
		}
	}
	for _, blocked := range []string{
		"system_user", "system_menu", "system_dict", "system_storage",
		"system_translation", "system_plugin", "system_error_log", "tools",
		"platform", "organization", "settings",
	} {
		for _, code := range codes {
			if code == blocked {
				t.Fatalf("allowlist must not list %q (directories or excluded pages)", blocked)
			}
		}
	}
}

// TestSyncTenantAdminRoleMenusAllowlistAgainstPostgres seeds a tenant_admin
// role with both allowlisted and forbidden menus, then checks boot sync keeps
// the former (plus ancestor directories and button children) and revokes the latter.
func TestSyncTenantAdminRoleMenusAllowlistAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping tenant_admin allowlist sync test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range []string{
		`CREATE SCHEMA tenant_admin_menu_probe`,
		`SET LOCAL search_path TO tenant_admin_menu_probe`,
		`CREATE TABLE sys_menu (
		   id BIGSERIAL PRIMARY KEY,
		   parent_id BIGINT NOT NULL DEFAULT 0,
		   type VARCHAR(16) NOT NULL,
		   name VARCHAR(128) NOT NULL,
		   code VARCHAR(128) NOT NULL,
		   status SMALLINT NOT NULL DEFAULT 1,
		   deleted_at TIMESTAMPTZ NULL
		 )`,
		`CREATE TABLE sys_role (
		   id BIGSERIAL PRIMARY KEY,
		   tenant_id BIGINT NOT NULL DEFAULT 1,
		   code VARCHAR(64) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL
		 )`,
		`CREATE TABLE sys_role_menu (
		   tenant_id BIGINT NOT NULL DEFAULT 1,
		   role_id BIGINT NOT NULL,
		   menu_id BIGINT NOT NULL,
		   PRIMARY KEY (role_id, menu_id)
		 )`,
		`INSERT INTO sys_menu (id, parent_id, type, name, code) VALUES
		   (10, 0, 'directory', '组织权限', 'organization'),
		   (11, 10, 'page', '用户管理', 'system_user'),
		   (14, 10, 'page', '角色管理', 'system_role'),
		   (16, 10, 'page', '成员管理', 'system_member'),
		   (15, 10, 'page', '菜单管理', 'system_menu'),
		   (70, 0, 'directory', '平台', 'platform'),
		   (71, 70, 'page', '租户', 'platform_tenants')`,
		`INSERT INTO sys_menu (id, parent_id, type, name, code) VALUES
		   (160, 16, 'button', '查询成员', 'system:member:query')`,
		`INSERT INTO sys_role (id, tenant_id, code) VALUES (9, 2, 'tenant_admin')`,
		`INSERT INTO sys_role_menu (tenant_id, role_id, menu_id) VALUES
		   (2, 9, 11), (2, 9, 14), (2, 9, 15), (2, 9, 16), (2, 9, 71)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare (%s): %v", statement, err)
		}
	}

	if err := syncTenantAdminRoleMenus(ctx, tx); err != nil {
		t.Fatalf("sync: %v", err)
	}

	assertGranted := func(code string, want bool) {
		t.Helper()
		var n int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sys_role_menu rm
INNER JOIN sys_menu m ON m.id = rm.menu_id
WHERE rm.role_id = 9 AND m.code = $1`, code).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", code, err)
		}
		got := n > 0
		if got != want {
			t.Fatalf("grant for %q = %v, want %v", code, got, want)
		}
	}

	assertGranted("system_member", true)
	assertGranted("system:member:query", true)
	assertGranted("system_role", true)
	assertGranted("organization", true) // ancestor directory
	assertGranted("system_user", false)
	assertGranted("system_menu", false)
	assertGranted("platform_tenants", false)
	assertGranted("platform", false)

	if err := grantTenantAdminRoleMenus(ctx, tx, 3, 9); err != nil {
		t.Fatalf("grant one role: %v", err)
	}
	assertGranted("system_member", true)
}
