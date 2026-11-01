package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestWrapDuplicatePostgresUniqueViolation(t *testing.T) {
	err := wrapDuplicate(&pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"})
	if !errors.Is(err, ErrDuplicateEntry) {
		t.Fatalf("expected ErrDuplicateEntry, got %v", err)
	}
}

func TestWrapDuplicateLeavesOtherErrorsUntouched(t *testing.T) {
	want := errors.New("connection unavailable")
	if got := wrapDuplicate(want); got != want {
		t.Fatalf("expected original error, got %v", got)
	}
}
