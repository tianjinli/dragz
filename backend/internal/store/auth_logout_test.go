package store

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

const revokeByHashSQL = `UPDATE sys_refresh_token SET revoked_at = NOW(), revoke_reason = $1
WHERE token_hash = $2 AND revoked_at IS NULL
RETURNING user_id`

const bumpTokenVersionSQL = `UPDATE sys_user SET token_version = token_version + 1, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`

// Logout must retire the exact session row so ListSessionsFiltered stops
// returning it, and must bump the user's token version in the same transaction
// so the access token issued by that login stops working immediately.
func TestRevokeRefreshTokenRetiresSessionAndBumpsVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(revokeByHashSQL)).
		WithArgs("logout", "token-hash").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(42))
	mock.ExpectExec(regexp.QuoteMeta(bumpTokenVersionSQL)).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewAuthRepository(&DB{DB: db})
	if err := repo.RevokeRefreshToken(context.Background(), "token-hash", "logout"); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

// A repeated logout (or a token that was already rotated) must not fail: the
// session is already absent from the online list and there is no version left
// to bump.
func TestRevokeRefreshTokenIsIdempotentWhenSessionAlreadyGone(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(revokeByHashSQL)).
		WithArgs("logout", "stale-hash").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	repo := NewAuthRepository(&DB{DB: db})
	if err := repo.RevokeRefreshToken(context.Background(), "stale-hash", "logout"); err != nil {
		t.Fatalf("RevokeRefreshToken: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
