package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestSeedRepairsIdSequencesAgainstPostgres covers the boot-time repair that
// keeps inserts from colliding with seeded rows.
//
// migrations/001_system_init.sql inserts its rows with explicit ids, so the
// table's sequence never advances: the first row created through the UI asks
// for id 1 again and fails with `duplicate key value violates unique constraint
// "sys_x_pkey"`. The repair is a DO block in seedStatements; this test runs the
// production statement against seeded-looking tables and checks the next id is
// free.
//
// Skipped unless DRAGZ_VERIFY_DSN points at a disposable database.
func TestSeedRepairsIdSequencesAgainstPostgres(t *testing.T) {
	dsn := os.Getenv("DRAGZ_VERIFY_DSN")
	if dsn == "" {
		t.Skip("DRAGZ_VERIFY_DSN not set; skipping the id-sequence repair test")
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
		`CREATE SCHEMA id_sequence_probe`,
		`SET LOCAL search_path TO id_sequence_probe`,
		// A table the seed filled with explicit ids: its sequence is still at 1.
		`CREATE TABLE seeded_rows (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`,
		`INSERT INTO seeded_rows (id, name) VALUES (1, 'a'), (2, 'b'), (3, 'c')`,
		// A table the seed left empty.
		`CREATE TABLE empty_rows (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("prepare schema (%s): %v", statement, err)
		}
	}

	// Before the repair, the next id collides with a seeded row. Proving that
	// keeps the test honest if someone later "simplifies" the seed.
	if _, err := tx.ExecContext(ctx, "SAVEPOINT before_repair"); err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO seeded_rows (name) VALUES ('collides')`); err == nil {
		t.Fatal("a fresh insert was accepted before the repair; the probe no longer reproduces the bug")
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT before_repair"); err != nil {
		t.Fatalf("rollback to savepoint: %v", err)
	}

	for _, statement := range seedStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed statement failed: %v\nstatement: %s", err, statement)
		}
	}

	var seededID, emptyID int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO seeded_rows (name) VALUES ('fresh') RETURNING id`).Scan(&seededID); err != nil {
		t.Fatalf("insert into the seeded table after the repair: %v", err)
	}
	if seededID <= 3 {
		t.Fatalf("the repaired sequence handed out id %d, which is already taken by a seeded row", seededID)
	}
	if err := tx.QueryRowContext(ctx, `INSERT INTO empty_rows (name) VALUES ('fresh') RETURNING id`).Scan(&emptyID); err != nil {
		t.Fatalf("insert into the empty table after the repair: %v", err)
	}
	if emptyID < 1 {
		t.Fatalf("insert into an empty table got id %d", emptyID)
	}

	// Running the repair twice must not move anything backwards.
	for _, statement := range seedStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			t.Fatalf("second run of the seed statement failed: %v", err)
		}
	}
	var again int64
	if err := tx.QueryRowContext(ctx, `INSERT INTO seeded_rows (name) VALUES ('fresh-2') RETURNING id`).Scan(&again); err != nil {
		t.Fatalf("insert after the second repair: %v", err)
	}
	if again <= seededID {
		t.Fatalf("second repair moved the sequence backwards: got id %d after %d", again, seededID)
	}
}
