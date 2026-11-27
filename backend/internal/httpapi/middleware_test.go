package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/casbinx"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

func TestAuthenticateRejectsMissingBearer(t *testing.T) {
	ctx := &app.Context{}
	handler := authenticate(ctx, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	handler(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestAuthenticateAcceptsValidAccessToken(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tokens := security.NewTokenManager("test-secret", "dragz", 3600, 7200)
	pair, err := tokens.Issue(42, "admin", 0, 0)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	mock.ExpectQuery("SELECT id, department_id, username").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "department_id", "username", "password_hash", "display_name", "email", "mobile",
			"status", "is_super_admin", "must_change_password", "failed_login_count", "locked_until", "token_version", "avatar_file_id",
		}).AddRow(42, 1, "admin", "hash", "Admin", "", "", 1, 1, 0, 0, nil, 0, nil))

	ctx := &app.Context{Tokens: tokens, Auth: store.NewAuthRepository(&store.DB{DB: db})}
	var gotUserID uint64
	handler := authenticate(ctx, func(w http.ResponseWriter, r *http.Request) {
		userID, ok := currentUserID(r.Context())
		if !ok {
			t.Fatal("expected user id in context")
		}
		gotUserID = userID
		required, ok := ctxkey.MustChangePasswordFrom(r.Context())
		if !ok || required {
			t.Fatalf("must-change flag = %v, present = %v; want false, true", required, ok)
		}
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	request.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if gotUserID != 42 {
		t.Fatalf("user id = %d, want 42", gotUserID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestAuthenticateCarriesMustChangePasswordFlag(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	tokens := security.NewTokenManager("test-secret", "dragz", 3600, 7200)
	pair, err := tokens.Issue(42, "admin", 0, 0)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	mock.ExpectQuery("SELECT id, department_id, username").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "department_id", "username", "password_hash", "display_name", "email", "mobile",
			"status", "is_super_admin", "must_change_password", "failed_login_count", "locked_until", "token_version", "avatar_file_id",
		}).AddRow(42, 1, "admin", "hash", "Admin", "", "", 1, 1, 1, 0, nil, 0, nil))

	ctx := &app.Context{Tokens: tokens, Auth: store.NewAuthRepository(&store.DB{DB: db})}
	handler := authenticate(ctx, func(w http.ResponseWriter, r *http.Request) {
		required, ok := ctxkey.MustChangePasswordFrom(r.Context())
		if !ok || !required {
			t.Fatalf("must-change flag = %v, present = %v; want true, true", required, ok)
		}
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/profile", nil)
	request.Header.Set("Authorization", "Bearer "+pair.AccessToken)
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRequirePermissionRejectsMissingUser(t *testing.T) {
	ctx := &app.Context{}
	handler := requirePermission(ctx, "system:user:query", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	handler(recorder, request)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestRequirePermissionAllowsSuperAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	enforcer, err := casbinx.NewEnforcer()
	if err != nil {
		t.Fatalf("casbin: %v", err)
	}
	ctx := &app.Context{
		Auth:   store.NewAuthRepository(&store.DB{DB: db}),
		Casbin: enforcer,
	}
	mock.ExpectQuery("SELECT id, department_id, username").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "department_id", "username", "password_hash", "display_name", "email", "mobile",
			"status", "is_super_admin", "must_change_password", "failed_login_count", "locked_until", "token_version", "avatar_file_id",
		}).AddRow(1, 1, "admin", "hash", "Admin", "", "", 1, 1, 0, 0, nil, 0, nil))

	handler := requirePermission(ctx, "system:user:query", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	request = withUserContext(request, 1, "admin")
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRequirePermissionDeniesWithoutPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	enforcer, err := casbinx.NewEnforcer()
	if err != nil {
		t.Fatalf("casbin: %v", err)
	}
	if err := enforcer.ReplacePolicies([]casbinx.Policy{{RoleID: 2, PermissionCode: "system:role:query"}}); err != nil {
		t.Fatalf("replace policies: %v", err)
	}
	ctx := &app.Context{
		Auth:   store.NewAuthRepository(&store.DB{DB: db}),
		Casbin: enforcer,
	}
	mock.ExpectQuery("SELECT id, department_id, username").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "department_id", "username", "password_hash", "display_name", "email", "mobile",
			"status", "is_super_admin", "must_change_password", "failed_login_count", "locked_until", "token_version", "avatar_file_id",
		}).AddRow(9, 1, "user", "hash", "User", "", "", 1, 0, 0, 0, nil, 0, nil))
	mock.ExpectQuery("SELECT ur.role_id FROM sys_user_role").
		WithArgs(uint64(9), uint64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"role_id"}).AddRow(3))

	handler := requirePermission(ctx, "system:user:query", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	request = withUserContext(request, 9, "user")
	handler(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}

	var body envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != 40300 {
		t.Fatalf("code = %d, want 40300", body.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRequirePermissionUsesCasbinPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	enforcer, err := casbinx.NewEnforcer()
	if err != nil {
		t.Fatalf("casbin: %v", err)
	}
	if err := enforcer.ReplacePolicies([]casbinx.Policy{{RoleID: 5, PermissionCode: "system:user:query"}}); err != nil {
		t.Fatalf("replace policies: %v", err)
	}
	ctx := &app.Context{
		Auth:   store.NewAuthRepository(&store.DB{DB: db}),
		Casbin: enforcer,
	}
	mock.ExpectQuery("SELECT id, department_id, username").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "department_id", "username", "password_hash", "display_name", "email", "mobile",
			"status", "is_super_admin", "must_change_password", "failed_login_count", "locked_until", "token_version", "avatar_file_id",
		}).AddRow(7, 1, "operator", "hash", "Operator", "", "", 1, 0, 0, 0, nil, 0, nil))
	mock.ExpectQuery("SELECT ur.role_id FROM sys_user_role").
		WithArgs(uint64(7), uint64(0)).
		WillReturnRows(sqlmock.NewRows([]string{"role_id"}).AddRow(5))

	handler := requirePermission(ctx, "system:user:query", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	request = withUserContext(request, 7, "operator")
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRequirePasswordChangedBlocksProtectedRoutes(t *testing.T) {
	handler := requirePasswordChanged(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	request = request.WithContext(ctxkey.WithMustChangePassword(request.Context(), true))
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
	}
	var body envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != apierrors.CodePasswordChangeRequired {
		t.Fatalf("code = %d, want %d", body.Code, apierrors.CodePasswordChangeRequired)
	}
}

func TestRequirePasswordChangedAllowsBootstrapAndChange(t *testing.T) {
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/change-password"},
		{http.MethodGet, "/api/auth/profile"},
		{http.MethodGet, "/api/auth/codes"},
		{http.MethodGet, "/api/auth/menus"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			handler := requirePasswordChanged(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			request := httptest.NewRequest(tc.method, tc.path, nil)
			request = request.WithContext(ctxkey.WithMustChangePassword(request.Context(), true))
			recorder := httptest.NewRecorder()
			handler(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
		})
	}
}

func TestRequirePasswordChangedSkipsRequestsWithoutFlag(t *testing.T) {
	// API-token callers never carry the flag; they must keep working.
	handler := requirePasswordChanged(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/system/users", nil)
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}
