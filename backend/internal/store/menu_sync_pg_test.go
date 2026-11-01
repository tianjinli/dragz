package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"dragz-admin/backend/internal/menuperm"
)

// TestSyncPermissionMenusRevivesRetiredButtonsAgainstPostgres covers the two
// halves of the permission-button sync that a unit test cannot see:
//
//   - a first sight of a code inserts a button, and the super administrator
//     role is granted it;
//   - a route that disappears retires the button (soft delete) but keeps the
//     grants, and a route that comes back revives the SAME row, so the grants
//     still point at something real.
//
// The second half is what the partial unique index made delicate: with a code
// that tombstones keep, "INSERT ... ON CONFLICT (code)" would have created a
// second row and left the grants behind.
//
// Skipped unless DRAGZ_VERIFY_DSN points at a disposable database.
func TestSyncPermissionMenusRevivesRetiredButtonsAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the permission-button sync test")
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

	// Only the columns the sync touches; the real schema lives in
	// migrations/001_system_init.sql, which this test deliberately does not
	// require.
	for _, statement := range []string{
		`CREATE SCHEMA menu_sync_probe`,
		`SET LOCAL search_path TO menu_sync_probe`,
		`CREATE TABLE sys_menu (
		   id BIGSERIAL PRIMARY KEY,
		   parent_id BIGINT NOT NULL DEFAULT 0,
		   type VARCHAR(16) NOT NULL,
		   name VARCHAR(128) NOT NULL,
		   code VARCHAR(128) NOT NULL,
		   method VARCHAR(16) NOT NULL DEFAULT '',
		   path VARCHAR(255) NOT NULL DEFAULT '',
		   is_sensitive SMALLINT NOT NULL DEFAULT 0,
		   sort INT NOT NULL DEFAULT 0,
		   status SMALLINT NOT NULL DEFAULT 1,
		   visible SMALLINT NOT NULL DEFAULT 1,
		   plugin_id VARCHAR(16) NOT NULL DEFAULT '',
		   managed SMALLINT NOT NULL DEFAULT 0,
		   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		   deleted_at TIMESTAMPTZ NULL
		 )`,
		`CREATE UNIQUE INDEX uk_menu_code ON sys_menu (code) WHERE deleted_at IS NULL`,
		`CREATE TABLE sys_role_menu (
		   role_id BIGINT NOT NULL,
		   menu_id BIGINT NOT NULL,
		   PRIMARY KEY (role_id, menu_id)
		 )`,
		`CREATE TABLE sys_translation (
		   entity_key VARCHAR(191) NOT NULL,
		   lang VARCHAR(16) NOT NULL,
		   value TEXT NOT NULL,
		   PRIMARY KEY (entity_key, lang)
		 )`,
		`INSERT INTO sys_menu (parent_id, type, name, code) VALUES (0, 'page', '用户管理', 'system_user')`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare schema (%s): %v", statement, err)
		}
	}

	// Run the sync on the transaction's connection so the scratch schema and
	// the rollback apply to it.
	sync := func(buttons []menuperm.Button) error {
		return syncPermissionMenus(ctx, tx, buttons)
	}

	var pageID uint64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM sys_menu WHERE code = 'system_user'`).Scan(&pageID); err != nil {
		t.Fatalf("page id: %v", err)
	}

	button := menuperm.Button{
		Code:     "system:user:create",
		Name:     "新增用户",
		NameEn:   "Create user",
		Method:   "POST",
		Path:     "/api/system/users",
		ParentID: pageID,
		Sort:     1,
	}

	if err := sync([]menuperm.Button{button}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	var firstID uint64
	var managed, status int16
	if err := tx.QueryRowContext(ctx,
		`SELECT id, managed, status FROM sys_menu WHERE code = $1`, button.Code).
		Scan(&firstID, &managed, &status); err != nil {
		t.Fatalf("read inserted button: %v", err)
	}
	if managed != 1 || status != 1 {
		t.Fatalf("inserted button has managed=%d status=%d, want 1/1", managed, status)
	}
	var grants int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sys_role_menu WHERE role_id = 1 AND menu_id = $1`, firstID).Scan(&grants); err != nil {
		t.Fatalf("count grants: %v", err)
	}
	if grants != 1 {
		t.Fatalf("super administrator role was granted %d times, want 1", grants)
	}

	// The route disappears: the button is retired but keeps its grants.
	if err := sync(nil); err != nil {
		t.Fatalf("retire sync: %v", err)
	}
	var retiredAt *time.Time
	var retiredStatus int16
	if err := tx.QueryRowContext(ctx,
		`SELECT deleted_at, status FROM sys_menu WHERE id = $1`, firstID).
		Scan(&retiredAt, &retiredStatus); err != nil {
		t.Fatalf("read retired button: %v", err)
	}
	if retiredAt == nil || retiredStatus != 0 {
		t.Fatalf("retired button has deleted_at=%v status=%d, want a timestamp and 0", retiredAt, retiredStatus)
	}

	// The route comes back: the same row is revived, so the grant still counts.
	if err := sync([]menuperm.Button{button}); err != nil {
		t.Fatalf("re-add sync: %v", err)
	}
	var revivedID uint64
	var revivedAt *time.Time
	var revivedStatus int16
	if err := tx.QueryRowContext(ctx,
		`SELECT id, deleted_at, status FROM sys_menu WHERE code = $1`, button.Code).
		Scan(&revivedID, &revivedAt, &revivedStatus); err != nil {
		t.Fatalf("read revived button: %v", err)
	}
	if revivedID != firstID {
		t.Fatalf("re-added route created a new row (%d) instead of reviving %d; the grants would have been orphaned", revivedID, firstID)
	}
	if revivedAt != nil || revivedStatus != 1 {
		t.Fatalf("revived button has deleted_at=%v status=%d, want NULL and 1", revivedAt, revivedStatus)
	}
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sys_role_menu WHERE role_id = 1 AND menu_id = $1`, firstID).Scan(&grants); err != nil {
		t.Fatalf("recount grants: %v", err)
	}
	if grants != 1 {
		t.Fatalf("revived button lost its grant (count=%d)", grants)
	}

	// A code a live page owns is never hijacked.
	err = sync([]menuperm.Button{{Code: "system_user", Name: "冒名", ParentID: 0}})
	if !errors.Is(err, ErrMenuCodeTaken) {
		t.Fatalf("syncing a code owned by a page returned %v, want ErrMenuCodeTaken", err)
	}
}
