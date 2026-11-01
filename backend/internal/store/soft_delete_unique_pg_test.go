package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// softDeleteKeyCase describes one business key that has to become reusable once
// the row holding it is soft deleted.
type softDeleteKeyCase struct {
	// legacyDDL is the table as migrations/001_system_init.sql created it
	// before the unique constraints moved into partial indexes.
	legacyDDL string
	// insert adds a row with the key values bound to $1 (and $2 when the key
	// is composite).
	insert string
	// softDelete marks the row deleted, exactly like the generated
	// SoftDelete* statement does.
	softDelete string
	table      string
}

var softDeleteKeyCases = []softDeleteKeyCase{
	{
		table: "sys_department",
		legacyDDL: `CREATE TABLE sys_department (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(64) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_department_code UNIQUE (code)
		 )`,
		insert:     `INSERT INTO sys_department (code) VALUES ($1)`,
		softDelete: `UPDATE sys_department SET deleted_at = NOW() WHERE code = $1`,
	},
	{
		table: "sys_position",
		legacyDDL: `CREATE TABLE sys_position (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(64) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_position_code UNIQUE (code)
		 )`,
		insert:     `INSERT INTO sys_position (code) VALUES ($1)`,
		softDelete: `UPDATE sys_position SET deleted_at = NOW() WHERE code = $1`,
	},
	{
		table: "sys_user",
		legacyDDL: `CREATE TABLE sys_user (
		   id BIGSERIAL PRIMARY KEY,
		   username VARCHAR(64) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_user_username UNIQUE (username)
		 )`,
		insert:     `INSERT INTO sys_user (username) VALUES ($1)`,
		softDelete: `UPDATE sys_user SET deleted_at = NOW() WHERE username = $1`,
	},
	{
		table: "sys_role",
		legacyDDL: `CREATE TABLE sys_role (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(64) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_role_code UNIQUE (code)
		 )`,
		insert:     `INSERT INTO sys_role (code) VALUES ($1)`,
		softDelete: `UPDATE sys_role SET deleted_at = NOW() WHERE code = $1`,
	},
	{
		table: "sys_menu",
		legacyDDL: `CREATE TABLE sys_menu (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(128) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_menu_code UNIQUE (code)
		 )`,
		insert:     `INSERT INTO sys_menu (code) VALUES ($1)`,
		softDelete: `UPDATE sys_menu SET deleted_at = NOW() WHERE code = $1`,
	},
	{
		table: "sys_dict_type",
		legacyDDL: `CREATE TABLE sys_dict_type (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(128) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_dict_type_code UNIQUE (code)
		 )`,
		insert:     `INSERT INTO sys_dict_type (code) VALUES ($1)`,
		softDelete: `UPDATE sys_dict_type SET deleted_at = NOW() WHERE code = $1`,
	},
	{
		table: "sys_dict_item",
		legacyDDL: `CREATE TABLE sys_dict_item (
		   id BIGSERIAL PRIMARY KEY,
		   dict_type_id BIGINT NOT NULL,
		   value VARCHAR(255) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_dict_item_value UNIQUE (dict_type_id, value)
		 )`,
		insert:     `INSERT INTO sys_dict_item (dict_type_id, value) VALUES ($1, $2)`,
		softDelete: `UPDATE sys_dict_item SET deleted_at = NOW() WHERE dict_type_id = $1 AND value = $2`,
	},
	{
		table: "sys_scheduled_job",
		legacyDDL: `CREATE TABLE sys_scheduled_job (
		   id BIGSERIAL PRIMARY KEY,
		   code VARCHAR(128) NOT NULL,
		   deleted_at TIMESTAMPTZ NULL,
		   CONSTRAINT uk_scheduled_job_code UNIQUE (code)
		 )`,
		insert:     `INSERT INTO sys_scheduled_job (code) VALUES ($1)`,
		softDelete: `UPDATE sys_scheduled_job SET deleted_at = NOW() WHERE code = $1`,
	},
}

// TestSoftDeleteKeysAreReusableAgainstPostgres proves the upgrade a running
// deployment gets on its next boot: the old table constraints are dropped, the
// partial unique indexes take over, and a key that was soft deleted can be
// used again while two live rows still collide.
//
// The statements under test are the production ones (bootstrapStatements), not
// a copy, so a typo in the upgrade DDL fails here.
//
// Skipped unless DRAGZ_VERIFY_DSN points at a disposable database:
//
//	docker run -d --rm --name dragz-verify-pg \
//	  -e POSTGRES_USER=verify -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=verify \
//	  -p 25433:5432 postgres:16-alpine
//	DRAGZ_VERIFY_DSN='postgres://verify:verify@127.0.0.1:25433/verify?sslmode=disable' \
//	  go test ./internal/store -run TestSoftDeleteKeysAreReusableAgainstPostgres -v
func TestSoftDeleteKeysAreReusableAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the soft-delete upgrade test")
	}

	db, err := OpenPostgres(dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Everything happens in a scratch schema inside one transaction that is
	// rolled back, so the database is left exactly as it was found.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `CREATE SCHEMA soft_delete_upgrade`); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `SET LOCAL search_path TO soft_delete_upgrade`); err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	for _, tc := range softDeleteKeyCases {
		if _, err := tx.ExecContext(ctx, tc.legacyDDL); err != nil {
			t.Fatalf("%s: create legacy table: %v", tc.table, err)
		}
	}
	// The upgrade itself: every bootstrap statement that touches one of these
	// tables runs, in the same order the server runs them at startup.
	applied := 0
	for _, statement := range bootstrapStatements {
		for _, tc := range softDeleteKeyCases {
			if strings.Contains(statement, tc.table) {
				if _, err := tx.ExecContext(ctx, statement); err != nil {
					t.Fatalf("%s: apply bootstrap statement %q: %v", tc.table, statement, err)
				}
				applied++
				break
			}
		}
	}
	if applied < len(softDeleteKeyCases)*2 {
		t.Fatalf("only %d bootstrap statements matched the soft-delete tables; expected at least %d (one DROP CONSTRAINT and one CREATE INDEX per table)", applied, len(softDeleteKeyCases)*2)
	}

	for _, tc := range softDeleteKeyCases {
		key := []any{"reuse-me"}
		if tc.table == "sys_dict_item" {
			key = []any{7, "reuse-me"}
		}
		// The old constraint must be gone: leaving it in place is exactly the
		// bug being fixed.
		var constraintCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pg_constraint c
			JOIN pg_class t ON t.oid = c.conrelid
			JOIN pg_namespace n ON n.oid = t.relnamespace
			WHERE n.nspname = 'soft_delete_upgrade' AND t.relname = $1
			  AND c.contype = 'u'`, tc.table).Scan(&constraintCount); err != nil {
			t.Fatalf("%s: count unique constraints: %v", tc.table, err)
		}
		if constraintCount != 0 {
			t.Fatalf("%s still carries %d unique constraint(s); the partial index never took over", tc.table, constraintCount)
		}

		// create → delete → create with the same key
		if _, err := tx.ExecContext(ctx, tc.insert, key...); err != nil {
			t.Fatalf("%s: first insert: %v", tc.table, err)
		}
		if _, err := tx.ExecContext(ctx, tc.softDelete, key...); err != nil {
			t.Fatalf("%s: soft delete: %v", tc.table, err)
		}
		if _, err := tx.ExecContext(ctx, tc.insert, key...); err != nil {
			t.Fatalf("%s: recreating the soft-deleted key must succeed, got: %v", tc.table, err)
		}

		// …but a second LIVE row with the same key is still refused. The
		// failed insert poisons the transaction, so it runs inside a savepoint.
		if _, err := tx.ExecContext(ctx, "SAVEPOINT duplicate_probe"); err != nil {
			t.Fatalf("%s: savepoint: %v", tc.table, err)
		}
		if _, err := tx.ExecContext(ctx, tc.insert, key...); err == nil {
			t.Fatalf("%s: a second live row with the same key was accepted", tc.table)
		} else if !strings.Contains(err.Error(), "duplicate key value") {
			t.Fatalf("%s: expected a duplicate-key error, got: %v", tc.table, err)
		}
		if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT duplicate_probe"); err != nil {
			t.Fatalf("%s: rollback to savepoint: %v", tc.table, err)
		}
	}
}

// TestSoftDeleteUniqueIndexesHaveBootstrapDDL keeps schema/tables.sql and the
// runtime upgrade in sync: a table that gains a partial unique index without a
// matching bootstrap statement would work on a fresh database and fail on an
// existing one.
func TestSoftDeleteUniqueIndexesHaveBootstrapDDL(t *testing.T) {
	raw, err := os.ReadFile("../../schema/tables.sql")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	var partial []string
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "CREATE UNIQUE INDEX ") {
			continue
		}
		if !strings.Contains(trimmed, "WHERE deleted_at IS NULL") {
			continue
		}
		fields := strings.Fields(trimmed)
		partial = append(partial, fields[3])
	}
	if len(partial) == 0 {
		t.Fatal("no partial unique indexes found in schema/tables.sql")
	}

	bootstrap := strings.Join(bootstrapStatements, "\n")
	for _, name := range partial {
		if !strings.Contains(bootstrap, "DROP CONSTRAINT IF EXISTS "+name) {
			t.Errorf("schema defines partial unique index %q but bootstrapStatements never drops the old constraint %q", name, name)
		}
		if !strings.Contains(bootstrap, "CREATE UNIQUE INDEX IF NOT EXISTS "+name) {
			t.Errorf("schema defines partial unique index %q but bootstrapStatements never creates it on existing databases", name)
		}
	}
}
