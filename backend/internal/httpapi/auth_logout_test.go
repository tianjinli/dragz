package httpapi

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

const revokeByHashSQL = `UPDATE sys_refresh_token SET revoked_at = NOW(), revoke_reason = $1
WHERE token_hash = $2 AND revoked_at IS NULL
RETURNING user_id`

const bumpTokenVersionSQL = `UPDATE sys_user SET token_version = token_version + 1, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`

// Logout is public and cookie-based. It must retire the refresh session even
// when no Authorization header is attached, otherwise the row stays in the
// online-user list until the token expires.
func TestLogoutHandlerRevokesCookieSessionWithoutAccessToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	mock.MatchExpectationsInOrder(false)

	hash := security.HashOpaqueToken("cookie-refresh-token")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(revokeByHashSQL)).
		WithArgs("logout", hash).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
	mock.ExpectExec(regexp.QuoteMeta(bumpTokenVersionSQL)).
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	ctx := &app.Context{
		Config: app.ConfigView{RefreshCookieName: "ua_refresh_token"},
		Auth:   store.NewAuthRepository(&store.DB{DB: db}),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "ua_refresh_token", Value: "cookie-refresh-token"})
	rec := httptest.NewRecorder()

	logoutHandler(ctx)(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "ua_refresh_token" || cookies[0].MaxAge >= 0 {
		t.Fatalf("refresh cookie must be cleared, got %+v", cookies)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
