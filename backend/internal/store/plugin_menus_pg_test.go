package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"dragz-admin/backend/internal/menuperm"
)

// TestSyncPluginMenusAgainstPostgres covers what a unit test cannot see:
//
//   - a plugin menu is inserted WITHOUT an id, so the sequence allocates one
//     and two plugins cannot collide on the primary key;
//   - re-running the sync is idempotent and keeps the same row, so grants that
//     point at it stay valid;
//   - an administrator rename survives a re-sync;
//   - a code owned by another plugin is refused loudly instead of being
//     silently skipped, which was the whole point of moving menus out of the
//     migration.
//
// Skipped unless DRAGZ_VERIFY_DSN points at a disposable database.
func TestSyncPluginMenusAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the declared-menu sync test")
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

	setupMenuProbe(t, ctx, tx, "plugin_menu_sync_probe")
	// A core menu, seeded with an explicit id exactly like 001_system_init.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sys_menu (id, type, name, code) VALUES (10, 'directory', '设置', 'probe_core')`,
	); err != nil {
		t.Fatalf("seed core menu: %v", err)
	}

	declared := []menuperm.MenuNode{
		{Code: "probe_page", ParentCode: "probe_core", Name: "探针页", NameEn: "Probe Page",
			RouteName: "Probe", RoutePath: "/probe", Component: "/probe/index",
			Icon: "lucide:box", Sort: 5, PluginID: "probe"},
		{Code: "probe_child", ParentCode: "probe_page", Name: "子页", PluginID: "probe"},
	}
	ordered, err := menuperm.OrderMenus(declared)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, ordered); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	type row struct {
		id       uint64
		parentID uint64
		name     string
	}
	read := func(code string) row {
		t.Helper()
		var out row
		var id, parentID int64
		if err := tx.QueryRowContext(ctx,
			`SELECT id, parent_id, name FROM sys_menu WHERE code = $1`, code,
		).Scan(&id, &parentID, &out.name); err != nil {
			t.Fatalf("read %q: %v", code, err)
		}
		out.id, out.parentID = uint64(id), uint64(parentID)
		return out
	}

	page := read("probe_page")
	if page.id == 0 || page.id == 10 {
		t.Fatalf("page got id %d; the sequence must allocate it, not collide with the seeded 10", page.id)
	}
	if page.parentID != 10 {
		t.Fatalf("parent_id = %d, want the core menu 10", page.parentID)
	}
	child := read("probe_child")
	if child.parentID != page.id {
		t.Fatalf("child parent_id = %d, want %d", child.parentID, page.id)
	}

	// Idempotent: same rows, same ids, no duplicates.
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, ordered); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if again := read("probe_page"); again.id != page.id {
		t.Fatalf("re-sync changed the id: %d -> %d", page.id, again.id)
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sys_menu WHERE type <> 'button'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("menu rows = %d, want 3 (core + 2 declared)", count)
	}

	// The administrator's name wins over a later sync.
	if _, err := tx.ExecContext(ctx, `UPDATE sys_menu SET name = '管理员改名' WHERE code = 'probe_page'`); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, ordered); err != nil {
		t.Fatalf("third sync: %v", err)
	}
	if got := read("probe_page").name; got != "管理员改名" {
		t.Fatalf("name = %q, want the administrator's rename to survive", got)
	}

	// Another plugin may not take the code over.
	foreign := []menuperm.MenuNode{{Code: "probe_page", Name: "偷来的", PluginID: "thief"}}
	orderedForeign, err := menuperm.OrderMenus(foreign)
	if err != nil {
		t.Fatalf("order foreign: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"thief"}, orderedForeign); !errors.Is(err, ErrMenuCodeTaken) {
		t.Fatalf("foreign sync = %v, want ErrMenuCodeTaken", err)
	}

	// A core menu is owned by plugin '' and is equally off limits.
	core := []menuperm.MenuNode{{Code: "probe_core", Name: "偷来的", PluginID: "thief"}}
	orderedCore, err := menuperm.OrderMenus(core)
	if err != nil {
		t.Fatalf("order core: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"thief"}, orderedCore); !errors.Is(err, ErrMenuCodeTaken) {
		t.Fatalf("core sync = %v, want ErrMenuCodeTaken", err)
	}

	// A parent that does not exist is a typo, not a top level menu.
	orphan := []menuperm.MenuNode{{Code: "probe_orphan", ParentCode: "nope", Name: "孤儿", PluginID: "probe"}}
	orderedOrphan, err := menuperm.OrderMenus(orphan)
	if err != nil {
		t.Fatalf("order orphan: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, orderedOrphan); !errors.Is(err, ErrMenuParentMissing) {
		t.Fatalf("orphan sync = %v, want ErrMenuParentMissing", err)
	}
}

// The reference plugin still carries a hard-coded menu id from its first
// migration. Declaring the same code must TAKE THAT ROW OVER — same id, same
// grants — instead of inserting a second row or colliding with it. This is the
// upgrade path for every plugin written before menus became declarative, and
// it is why an existing installation does not need a fixing migration.
func TestSyncPluginMenusTakesOverHardCodedRowAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the hard-coded menu takeover test")
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

	setupMenuProbe(t, ctx, tx, "plugin_menu_takeover_probe")

	// Exactly what plugins/example/migrations/001_init.sql leaves behind, plus
	// the grant that makes the id worth preserving.
	for _, statement := range []string{
		`INSERT INTO sys_menu
		   (id, type, name, code, route_name, route_path, component, icon, sort, plugin_id)
		 VALUES (1000, 'page', '示例插件', 'example_page', 'ExamplePage', '/example',
		         '/example/index', 'lucide:box', 900, 'example')`,
		`INSERT INTO sys_role_menu (role_id, menu_id) VALUES (1, 1000)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}

	declared := []menuperm.MenuNode{{
		Code:      "example_page",
		Name:      "示例插件",
		NameEn:    "Example Plugin",
		RouteName: "ExamplePage",
		RoutePath: "/example",
		Component: "/example/index",
		Icon:      "lucide:box",
		Sort:      900,
		PluginID:  "example",
	}}
	ordered, err := menuperm.OrderMenus(declared)
	if err != nil {
		t.Fatalf("order: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"example"}, ordered); err != nil {
		t.Fatalf("sync onto the hard-coded row: %v", err)
	}

	var count int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sys_menu WHERE code = 'example_page'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("example_page rows = %d, want 1 (takeover, not a second row)", count)
	}

	var id int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM sys_menu WHERE code = 'example_page'`).Scan(&id); err != nil {
		t.Fatalf("read id: %v", err)
	}
	if id != 1000 {
		t.Fatalf("id = %d, want the original 1000 so grants keep pointing at it", id)
	}

	var granted bool
	if err := tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM sys_role_menu WHERE role_id = 1 AND menu_id = 1000)`).Scan(&granted); err != nil {
		t.Fatalf("read grant: %v", err)
	}
	if !granted {
		t.Fatal("the super administrator grant was lost")
	}
}

// setupMenuProbe builds the slice of the real schema the menu sync touches.
// The real DDL lives in migrations/001_system_init.sql, which these tests
// deliberately do not require.
func setupMenuProbe(t *testing.T, ctx context.Context, tx *Tx, schema string) {
	t.Helper()
	for _, statement := range []string{
		`CREATE SCHEMA ` + schema,
		`SET LOCAL search_path TO ` + schema,
		`CREATE TABLE sys_menu (
		   id BIGSERIAL PRIMARY KEY,
		   parent_id BIGINT NOT NULL DEFAULT 0,
		   type VARCHAR(16) NOT NULL,
		   name VARCHAR(128) NOT NULL,
		   code VARCHAR(128) NOT NULL,
		   route_name VARCHAR(128) NOT NULL DEFAULT '',
		   route_path VARCHAR(255) NOT NULL DEFAULT '',
		   component VARCHAR(255) NOT NULL DEFAULT '',
		   redirect VARCHAR(255) NOT NULL DEFAULT '',
		   icon VARCHAR(128) NOT NULL DEFAULT '',
		   sort INT NOT NULL DEFAULT 0,
		   status SMALLINT NOT NULL DEFAULT 1,
		   visible SMALLINT NOT NULL DEFAULT 1,
		   keep_alive SMALLINT NOT NULL DEFAULT 0,
		   external_url VARCHAR(500) NOT NULL DEFAULT '',
		   plugin_id VARCHAR(16) NOT NULL DEFAULT '',
		   method VARCHAR(16) NOT NULL DEFAULT '',
		   path VARCHAR(255) NOT NULL DEFAULT '',
		   is_sensitive SMALLINT NOT NULL DEFAULT 0,
		   managed SMALLINT NOT NULL DEFAULT 0,
		   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		   deleted_at TIMESTAMPTZ NULL
		 )`,
		`CREATE UNIQUE INDEX uk_menu_code ON sys_menu (code) WHERE deleted_at IS NULL`,
		`CREATE TABLE sys_translation (
		   entity_key VARCHAR(255) NOT NULL,
		   lang VARCHAR(16) NOT NULL,
		   value TEXT NOT NULL,
		   PRIMARY KEY (entity_key, lang)
		 )`,
		`CREATE TABLE sys_role_menu (
		   role_id BIGINT NOT NULL,
		   menu_id BIGINT NOT NULL,
		   PRIMARY KEY (role_id, menu_id)
		 )`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("setup %q: %v", statement, err)
		}
	}
}

// Dropping a menu from the declaration must retire the row, and putting it back
// must revive the SAME row so the grants that point at it survive. Without this
// the sidebar would keep offering an entry whose component no longer exists.
func TestSyncPluginMenusRetiresAndRevivesAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the menu retirement test")
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

	setupMenuProbe(t, ctx, tx, "plugin_menu_retire_probe")

	both := []menuperm.MenuNode{
		{Code: "keep_page", Name: "保留", PluginID: "probe"},
		{Code: "drop_page", Name: "待删", PluginID: "probe"},
	}
	orderedBoth, err := menuperm.OrderMenus(both)
	if err != nil {
		t.Fatalf("order both: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, orderedBoth); err != nil {
		t.Fatalf("first sync: %v", err)
	}

	var dropID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM sys_menu WHERE code = 'drop_page' AND deleted_at IS NULL`).Scan(&dropID); err != nil {
		t.Fatalf("read drop_page: %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sys_role_menu (role_id, menu_id) VALUES (1, $1)`, dropID); err != nil {
		t.Fatalf("grant drop_page: %v", err)
	}

	// Drop it from the declaration: the row is retired, the grant is kept.
	onlyKeep, err := menuperm.OrderMenus([]menuperm.MenuNode{{Code: "keep_page", Name: "保留", PluginID: "probe"}})
	if err != nil {
		t.Fatalf("order keep: %v", err)
	}
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, onlyKeep); err != nil {
		t.Fatalf("retiring sync: %v", err)
	}
	var retiredAt *time.Time
	var status int64
	if err := tx.QueryRowContext(ctx,
		`SELECT deleted_at, status FROM sys_menu WHERE code = 'drop_page'`).Scan(&retiredAt, &status); err != nil {
		t.Fatalf("read retired row: %v", err)
	}
	if retiredAt == nil {
		t.Fatal("drop_page was not retired")
	}
	if status != 0 {
		t.Fatalf("retired status = %d, want 0", status)
	}
	var grants int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sys_role_menu WHERE menu_id = $1`, dropID).Scan(&grants); err != nil {
		t.Fatalf("count grants: %v", err)
	}
	if grants != 1 {
		t.Fatalf("grants = %d, want the grant kept for a possible return", grants)
	}

	// Put it back: same id, same grant, live again.
	if err := syncPluginMenus(ctx, tx, []string{"probe"}, orderedBoth); err != nil {
		t.Fatalf("reviving sync: %v", err)
	}
	var revivedID int64
	var revivedAt *time.Time
	if err := tx.QueryRowContext(ctx,
		`SELECT id, deleted_at FROM sys_menu WHERE code = 'drop_page'`).Scan(&revivedID, &revivedAt); err != nil {
		t.Fatalf("read revived row: %v", err)
	}
	if revivedAt != nil {
		t.Fatal("drop_page was not revived")
	}
	if revivedID != dropID {
		t.Fatalf("revived id = %d, want the original %d so the grant still applies", revivedID, dropID)
	}
	var rows int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sys_menu WHERE code = 'drop_page'`).Scan(&rows); err != nil {
		t.Fatalf("count drop_page: %v", err)
	}
	if rows != 1 {
		t.Fatalf("drop_page rows = %d, want 1 (revived, not re-inserted)", rows)
	}
}
