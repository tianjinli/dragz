package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestUninstallPluginAgainstPostgres runs the real DDL against a real server.
//
// The unit test above proves the validator rejects bad identifiers, but the
// DROP/DELETE statements themselves are only exercised here: sys_menu,
// sys_role_menu and sys_plugin are created by runtime bootstrap DDL, so a
// column rename would break uninstall without any unit test noticing.
//
// It is skipped unless DRAGZ_VERIFY_DSN points at a disposable database, so a
// normal `go test ./...` stays hermetic:
//
//	docker run -d --rm --name dragz-verify-pg \
//	  -e POSTGRES_USER=verify -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=verify \
//	  -p 25433:5432 postgres:16-alpine
//	DRAGZ_VERIFY_DSN='postgres://verify:verify@127.0.0.1:25433/verify?sslmode=disable' \
//	  go test ./internal/store -run TestUninstallPluginAgainstPostgres -v
func TestUninstallPluginAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the real-database uninstall test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Only the columns UninstallPlugin touches; the real schema comes from
	// migrations/001_init.sql, which this test deliberately does not require.
	// demo_ref is dropped first: the sibling rollback test leaves it behind with
	// a foreign key to demo_item, which would block this DROP on a second run.
	schema := []string{
		`DROP TABLE IF EXISTS demo_ref, sys_role_menu, sys_menu, sys_plugin_migration, sys_plugin, demo_item`,
		`CREATE TABLE sys_plugin (
		   plugin_id VARCHAR(16) PRIMARY KEY,
		   enabled BOOLEAN NOT NULL DEFAULT FALSE
		 )`,
		`CREATE TABLE sys_plugin_migration (
		   plugin_id VARCHAR(16) NOT NULL,
		   version VARCHAR(32) NOT NULL,
		   PRIMARY KEY (plugin_id, version)
		 )`,
		`CREATE TABLE sys_menu (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(64) NOT NULL DEFAULT '',
		   plugin_id VARCHAR(16) NOT NULL DEFAULT ''
		 )`,
		`CREATE TABLE sys_role_menu (
		   role_id BIGINT NOT NULL,
		   menu_id BIGINT NOT NULL,
		   PRIMARY KEY (role_id, menu_id)
		 )`,
		`CREATE TABLE demo_item (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`,
	}
	for _, statement := range schema {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare schema: %v\nstatement: %s", err, statement)
		}
	}

	seed := []string{
		`INSERT INTO sys_plugin (plugin_id, enabled) VALUES ('demo', FALSE)`,
		`INSERT INTO sys_plugin_migration (plugin_id, version) VALUES ('demo', '001')`,
		`INSERT INTO sys_menu (code, plugin_id) VALUES ('demo_page', 'demo')`,
		`INSERT INTO sys_menu (code, plugin_id) VALUES ('demo:item:create', 'demo')`,
		`INSERT INTO sys_menu (code, plugin_id) VALUES ('system_user', '')`,
		`INSERT INTO sys_role_menu (role_id, menu_id) SELECT 1, id FROM sys_menu WHERE plugin_id = 'demo'`,
		`INSERT INTO demo_item (name) VALUES ('hello')`,
	}
	for _, statement := range seed {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed: %v\nstatement: %s", err, statement)
		}
	}

	repo := NewPluginRepository(db)
	if err := repo.UninstallPlugin(ctx, "demo", []string{"demo_item"}); err != nil {
		t.Fatalf("UninstallPlugin: %v", err)
	}

	if tableExists(t, ctx, db, "demo_item") {
		t.Error("demo_item still exists; DROP TABLE did not run")
	}
	assertCount(t, ctx, db, `SELECT COUNT(*) FROM sys_plugin WHERE plugin_id = 'demo'`, 0, "catalog row")
	assertCount(t, ctx, db, `SELECT COUNT(*) FROM sys_plugin_migration WHERE plugin_id = 'demo'`, 0, "migration rows")
	assertCount(t, ctx, db, `SELECT COUNT(*) FROM sys_menu WHERE plugin_id = 'demo'`, 0, "plugin menus")
	assertCount(t, ctx, db, `SELECT COUNT(*) FROM sys_role_menu`, 0, "grants of the plugin's menus")
	// Core menus must survive: the delete is scoped by plugin_id.
	assertCount(t, ctx, db, `SELECT COUNT(*) FROM sys_menu WHERE plugin_id = ''`, 1, "core menus")
}

// A table still referenced by another plugin's foreign key must abort the whole
// transaction, leaving the catalog intact rather than half uninstalled.
func TestUninstallPluginRollsBackOnForeignKey(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the real-database uninstall test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Rebuild a minimal schema: the previous test may have left one behind.
	for _, statement := range []string{
		`DROP TABLE IF EXISTS demo_ref, demo_item, sys_role_menu, sys_menu, sys_plugin_migration, sys_plugin`,
		`CREATE TABLE sys_plugin (plugin_id VARCHAR(16) PRIMARY KEY, enabled BOOLEAN NOT NULL DEFAULT FALSE)`,
		`CREATE TABLE sys_plugin_migration (plugin_id VARCHAR(16) NOT NULL, version VARCHAR(32) NOT NULL, PRIMARY KEY (plugin_id, version))`,
		`CREATE TABLE sys_menu (id BIGSERIAL PRIMARY KEY, code VARCHAR(64) NOT NULL DEFAULT '', plugin_id VARCHAR(16) NOT NULL DEFAULT '')`,
		`CREATE TABLE sys_role_menu (role_id BIGINT NOT NULL, menu_id BIGINT NOT NULL, PRIMARY KEY (role_id, menu_id))`,
		`CREATE TABLE demo_item (id BIGSERIAL PRIMARY KEY)`,
		`CREATE TABLE demo_ref (id BIGSERIAL PRIMARY KEY, item_id BIGINT NOT NULL REFERENCES demo_item (id))`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare schema: %v\nstatement: %s", err, statement)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sys_plugin (plugin_id) VALUES ('demo')`); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	repo := NewPluginRepository(db)
	if err := repo.UninstallPlugin(ctx, "demo", []string{"demo_item"}); err == nil {
		t.Fatal("want a foreign-key error, got nil")
	}
	// PostgreSQL runs DDL transactionally, so nothing may have been removed.
	assertCount(t, ctx, db, `SELECT COUNT(*) FROM sys_plugin WHERE plugin_id = 'demo'`, 1, "catalog row after rollback")
	if !tableExists(t, ctx, db, "demo_item") {
		t.Error("demo_item was dropped even though the transaction failed")
	}
}

func tableExists(t *testing.T, ctx context.Context, db *DB, name string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('public.'||$1) IS NOT NULL`, name).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", name, err)
	}
	return exists
}

func assertCount(t *testing.T, ctx context.Context, db *DB, query string, want int, what string) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(ctx, query).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", what, err)
	}
	if got != want {
		t.Errorf("%s = %d, want %d", what, got, want)
	}
}
