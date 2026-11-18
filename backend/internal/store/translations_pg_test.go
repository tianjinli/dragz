package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// TestTranslationDeletesReportWhatTheyRemoved runs the real DELETE statements
// against a real server.
//
// The delete endpoint addresses a translation by entity key, and the page shows
// one row per preset entity — so a key that no longer has a record is the normal
// case after another operator clears it. The handler decides 200 or 404 on the
// row count, which only the real statement can produce; a fake would happily
// report a deletion that never happened.
//
// It is skipped unless DRAGZ_VERIFY_DSN points at a disposable database, so a
// normal `go test ./...` stays hermetic:
//
//	docker run -d --rm --name dragz-verify-pg \
//	  -e POSTGRES_USER=verify -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=verify \
//	  -p 25433:5432 postgres:16-alpine
//	DRAGZ_VERIFY_DSN='postgres://verify:verify@127.0.0.1:25433/verify?sslmode=disable' \
//	  go test ./internal/store -run TestTranslationDeletesReportWhatTheyRemoved -v
func TestTranslationDeletesReportWhatTheyRemoved(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the real-database translation test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Only the columns the two statements touch; migrations/001_system_init.sql
	// is deliberately not required here.
	schema := []string{
		`DROP TABLE IF EXISTS sys_translation`,
		`CREATE TABLE sys_translation (
		   id BIGSERIAL PRIMARY KEY,
		   entity_key VARCHAR(128) NOT NULL,
		   lang VARCHAR(16) NOT NULL,
		   value TEXT NOT NULL,
		   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		   UNIQUE (entity_key, lang)
		 )`,
	}
	for _, stmt := range schema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	repo := NewAuthRepository(db)
	if err := repo.CreateTranslation(ctx, "menu.dashboard", "en", "Workbench"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First delete removes the row, second one finds nothing left. The page
	// turns the second answer into 404 instead of pretending it deleted a row.
	removed, err := repo.DeleteTranslationForEntity(ctx, "menu.dashboard", "en")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !removed {
		t.Fatal("first delete reported no removal, want true")
	}
	removed, err = repo.DeleteTranslationForEntity(ctx, "menu.dashboard", "en")
	if err != nil {
		t.Fatalf("second delete: %v", err)
	}
	if removed {
		t.Fatal("second delete reported a removal, want false")
	}

	// The same statement is used to clear a language on save, where "nothing to
	// delete" must stay a success: an entity with no text in this language is a
	// normal state, not an error.
	if err := repo.CreateTranslation(ctx, "menu.files", "en", "Files"); err != nil {
		t.Fatalf("create: %v", err)
	}
	var id int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM sys_translation WHERE entity_key = 'menu.files' AND lang = 'en'`).Scan(&id); err != nil {
		t.Fatalf("select id: %v", err)
	}
	if err := repo.UpdateTranslation(ctx, id, "en", ""); err != nil {
		t.Fatalf("clear existing: %v", err)
	}
	// Clearing again must say the record is gone rather than answer ok: an edit
	// addressed by a record id has nothing left to write.
	if err := repo.UpdateTranslation(ctx, id, "en", ""); !errors.Is(err, ErrTranslationNotFound) {
		t.Fatalf("clear missing = %v, want ErrTranslationNotFound", err)
	}
}

// TestListEntityTranslationsFiltersByNamespace covers the list page's group
// filter: a namespace keeps only matching entity_key prefixes and the window
// total reflects that filter, not the unfiltered catalog.
//
// Skipped unless DRAGZ_VERIFY_DSN is set — same disposable-database rule as
// TestTranslationDeletesReportWhatTheyRemoved.
func TestListEntityTranslationsFiltersByNamespace(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the real-database translation test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schema := []string{
		`DROP VIEW IF EXISTS v_preset_entity`,
		`DROP TABLE IF EXISTS sys_translation`,
		`CREATE TABLE sys_translation (
		   id BIGSERIAL PRIMARY KEY,
		   entity_key VARCHAR(128) NOT NULL,
		   lang VARCHAR(16) NOT NULL,
		   value TEXT NOT NULL,
		   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		   UNIQUE (entity_key, lang)
		 )`,
		`CREATE VIEW v_preset_entity AS
		   SELECT * FROM (VALUES
		     ('menu.dashboard'::text, '工作台'::text, NOW()),
		     ('menu.system', '系统管理', NOW()),
		     ('role.admin', '管理员', NOW())
		   ) AS t(entity_key, source, updated_at)`,
	}
	for _, stmt := range schema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}

	repo := NewAuthRepository(db)
	if err := repo.CreateTranslation(ctx, "menu.dashboard", "en", "Workbench"); err != nil {
		t.Fatalf("create: %v", err)
	}

	items, total, err := repo.ListEntityTranslations(ctx, "en", "", "menu", 1, 20)
	if err != nil {
		t.Fatalf("list menu: %v", err)
	}
	if total != 2 {
		t.Fatalf("menu total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Fatalf("menu items = %d, want 2", len(items))
	}
	for _, item := range items {
		if len(item.EntityKey) < 5 || item.EntityKey[:5] != "menu." {
			t.Fatalf("unexpected entity key %q under menu filter", item.EntityKey)
		}
	}
	if items[0].Value != "Workbench" {
		t.Fatalf("dashboard value = %q, want Workbench", items[0].Value)
	}
	if items[1].Value != "" {
		t.Fatalf("system value = %q, want empty", items[1].Value)
	}

	items, total, err = repo.ListEntityTranslations(ctx, "en", "", "role", 1, 20)
	if err != nil {
		t.Fatalf("list role: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].EntityKey != "role.admin" {
		t.Fatalf("role filter = total %d items %+v, want one role.admin", total, items)
	}

	_, total, err = repo.ListEntityTranslations(ctx, "en", "", "", 1, 20)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if total != 3 {
		t.Fatalf("all total = %d, want 3", total)
	}
}
