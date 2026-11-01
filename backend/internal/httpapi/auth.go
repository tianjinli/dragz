// Package httpapi is the HTTP layer: route declarations, thin handlers, and
// the middleware chain (localize, plugin gate, authenticate, audit, guard).
package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/httpapi/logic"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/internal/validation"
)

const dummyPasswordHash = "$2a$10$4N0mDjsKXQ16kQOyJVXMV.NfP5jgv5FQLm9K3dU8Jq4jf9fctG/Ju"

type loginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	CaptchaID   string `json:"captchaId"`
	CaptchaCode string `json:"captchaCode"`
	DeviceID    string `json:"deviceId"`
	DeviceName  string `json:"deviceName"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
	DeviceID     string `json:"deviceId"`
	DeviceName   string `json:"deviceName"`
}

type logoutRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type tokenResponse struct {
	AccessToken      string `json:"accessToken"`
	TokenType        string `json:"tokenType"`
	ExpiresIn        int64  `json:"expiresIn"`
	RefreshExpiresIn int64  `json:"refreshExpiresIn"`
}

type profileResponse struct {
	ID                 uint64   `json:"id"`
	UserID             string   `json:"userId"`
	Username           string   `json:"username"`
	RealName           string   `json:"realName"`
	Email              string   `json:"email"`
	Mobile             string   `json:"mobile"`
	Avatar             string   `json:"avatar"`
	DepartmentName     string   `json:"departmentName"`
	Roles              []string `json:"roles"`
	RoleNames          []string `json:"roleNames"`
	Permissions        []string `json:"permissions"`
	HomePath           string   `json:"homePath"`
	IsSuperAdmin       bool     `json:"isSuperAdmin"`
	MustChangePassword bool     `json:"mustChangePassword"`
	PasswordMinLength  int      `json:"passwordMinLength"`
}

type routeNode struct {
	ID        uint64       `json:"-"`
	ParentID  uint64       `json:"-"`
	Path      string       `json:"path"`
	Name      string       `json:"name"`
	Component string       `json:"component,omitempty"`
	Redirect  string       `json:"redirect,omitempty"`
	Meta      routeMeta    `json:"meta"`
	Children  []*routeNode `json:"children,omitempty"`
}

type routeMeta struct {
	Title       string `json:"title"`
	Icon        string `json:"icon,omitempty"`
	Order       int    `json:"order"`
	HideInMenu  bool   `json:"hideInMenu,omitempty"`
	KeepAlive   bool   `json:"keepAlive,omitempty"`
	AffixTab    bool   `json:"affixTab,omitempty"`
	TabClosable *bool  `json:"tabClosable,omitempty"`
	Link        string `json:"link,omitempty"`
}

func loginHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if !decode(w, r, &req) {
			return
		}
		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" {
			fail(w, r, http.StatusBadRequest, 40001, "errors.username_password_required")
			return
		}

		authLogic := logic.NewAuthLogic(logicContext(ctx))
		securitySettings, settingsErr := authLogic.LoginSecuritySettings(r.Context())
		if settingsErr != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
			return
		}

		user, err := authLogic.FindUserByUsername(r.Context(), req.Username)
		failedCountHint := 0
		if err == nil {
			failedCountHint = user.FailedLoginCount
		} else if errors.Is(err, sql.ErrNoRows) {
			failedCountHint = unknownFailCount(ctx, r.Context(), req.Username)
		}
		needCaptcha := shouldRequireCaptcha(securitySettings.CaptchaEnabled, failedCountHint, hasCaptchaNeedFlag(ctx, r.Context(), req.Username))
		if needCaptcha {
			if verifyErr := verifyAndConsumeCaptcha(ctx, r.Context(), req.CaptchaID, req.CaptchaCode); verifyErr != nil {
				msg := "errors.captcha_required"
				if errors.Is(verifyErr, errCaptchaInvalid) {
					msg = "errors.captcha_invalid"
				}
				until := time.Now().Add(loginFailureCoolDown)
				failData(w, r, http.StatusBadRequest, 40002, msg, newLoginLockData(until, loginFailureCoolDown, false, failedCountHint, securitySettings.MaxAttempts, true))
				return
			}
		}

		if errors.Is(err, sql.ErrNoRows) {
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(req.Password))
			_ = ctx.Auth.RecordLoginFailure(r.Context(), req.Username, clientIP(r), r.UserAgent(), "login_log.reason.invalid_credentials", nil)
			nextCount := bumpUnknownFailCount(ctx, r.Context(), req.Username)
			captchaRequired := noteCaptchaAfterFailure(ctx, r.Context(), req.Username, securitySettings.CaptchaEnabled, nextCount)
			until, duration, _ := resolveLoginFailurePenalty(0, securitySettings)
			failData(w, r, http.StatusUnauthorized, 40101, "errors.invalid_login", newLoginLockData(until, duration, false, nextCount, securitySettings.MaxAttempts, captchaRequired))
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
			return
		}
		if user.Status != 1 {
			_ = ctx.Auth.RecordLoginFailure(r.Context(), req.Username, clientIP(r), r.UserAgent(), "login_log.reason.account_disabled", nil)
			fail(w, r, http.StatusForbidden, 40301, "errors.account_disabled")
			return
		}
		if user.LockedUntil.Valid && user.LockedUntil.Time.After(time.Now()) {
			remaining := time.Until(user.LockedUntil.Time)
			if shouldEnforceLoginWait(remaining) {
				locked := isAccountLockWait(user.FailedLoginCount, securitySettings.MaxAttempts, remaining)
				msg := "errors.retry_later"
				status, code := http.StatusUnauthorized, 40101
				if locked {
					msg = "errors.locked"
					status, code = http.StatusLocked, 42301
				}
				captchaRequired := shouldRequireCaptcha(securitySettings.CaptchaEnabled, user.FailedLoginCount, hasCaptchaNeedFlag(ctx, r.Context(), req.Username))
				failData(w, r, status, code, msg, newLoginLockData(user.LockedUntil.Time, remaining, locked, user.FailedLoginCount, securitySettings.MaxAttempts, captchaRequired))
				return
			}
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
			nextCount := user.FailedLoginCount + 1
			until, duration, locked := resolveLoginFailurePenalty(nextCount, securitySettings)
			_ = ctx.Auth.RecordLoginFailure(r.Context(), req.Username, clientIP(r), r.UserAgent(), "login_log.reason.invalid_credentials", &until)
			captchaRequired := noteCaptchaAfterFailure(ctx, r.Context(), req.Username, securitySettings.CaptchaEnabled, nextCount)
			msg, msgData := loginFailureMessageKey(nextCount, locked, securitySettings.MaxAttempts)
			data := newLoginLockData(until, duration, locked, nextCount, securitySettings.MaxAttempts, captchaRequired)
			if locked {
				failData(w, r, http.StatusLocked, 42301, msg, data, msgData)
				return
			}
			failData(w, r, http.StatusUnauthorized, 40101, msg, data, msgData)
			return
		}

		clearCaptchaRequired(ctx, r.Context(), req.Username)
		pair, err := ctx.Tokens.Issue(user.ID, user.Username, user.TokenVersion)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.token_issue_failed")
			return
		}
		if err := ctx.Auth.SaveRefreshToken(r.Context(), user.ID, pair.RefreshHash, pair.RefreshFamilyID,
			req.DeviceID, req.DeviceName, clientIP(r), r.UserAgent(), pair.RefreshExpiresAt); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.session_create_failed")
			return
		}
		_ = ctx.Auth.RecordLoginSuccess(r.Context(), user.ID, user.Username, clientIP(r), r.UserAgent())
		writeRefreshCookie(w, ctx, pair.RefreshToken)
		ok(w, r, pairResponse(ctx, pair))
	}
}

func refreshHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshRequest
		if r.ContentLength > 0 && !decode(w, r, &req) {
			return
		}
		if req.RefreshToken == "" {
			if cookie, cookieErr := r.Cookie(ctx.Config.RefreshCookieName); cookieErr == nil {
				req.RefreshToken = cookie.Value
			}
		}
		if req.RefreshToken == "" {
			fail(w, r, http.StatusUnauthorized, 40102, "errors.refresh_invalid")
			return
		}
		hash := security.HashOpaqueToken(req.RefreshToken)
		session, err := ctx.Auth.FindRefreshSession(r.Context(), hash)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, 40102, "errors.refresh_invalid")
			return
		}
		if session.RevokedAt.Valid {
			_ = ctx.Auth.RevokeRefreshFamily(r.Context(), session.FamilyID, "refresh token reuse detected")
			fail(w, r, http.StatusUnauthorized, 40103, "errors.refresh_reuse")
			return
		}
		if !session.ExpiresAt.After(time.Now()) {
			_ = ctx.Auth.RevokeRefreshToken(r.Context(), hash, "expired")
			fail(w, r, http.StatusUnauthorized, 40104, "errors.refresh_expired")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), session.UserID)
		if err != nil || user.Status != 1 {
			_ = ctx.Auth.RevokeRefreshFamily(r.Context(), session.FamilyID, "user disabled or missing")
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		pair, err := ctx.Tokens.Issue(session.UserID, session.Username, user.TokenVersion)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.token_issue_failed")
			return
		}
		pair.RefreshFamilyID = session.FamilyID
		if err := ctx.Auth.RotateRefreshToken(r.Context(), session.ID, session.UserID, pair.RefreshHash,
			pair.RefreshFamilyID, req.DeviceID, req.DeviceName, clientIP(r), r.UserAgent(), pair.RefreshExpiresAt); err != nil {
			_ = ctx.Auth.RevokeRefreshFamily(r.Context(), session.FamilyID, "rotation failed")
			fail(w, r, http.StatusUnauthorized, 40103, "errors.refresh_used")
			return
		}
		writeRefreshCookie(w, ctx, pair.RefreshToken)
		ok(w, r, pairResponse(ctx, pair))
	}
}

func logoutHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req logoutRequest
		if r.ContentLength > 0 && !decode(w, r, &req) {
			return
		}
		if req.RefreshToken == "" {
			if cookie, cookieErr := r.Cookie(ctx.Config.RefreshCookieName); cookieErr == nil {
				req.RefreshToken = cookie.Value
			}
		}
		if req.RefreshToken != "" {
			_ = ctx.Auth.RevokeRefreshToken(r.Context(), security.HashOpaqueToken(req.RefreshToken), "logout")
		}
		clearRefreshCookie(w, ctx)
		ok(w, r, map[string]bool{"success": true})
	}
}

func profileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil || user.Status != 1 {
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		roles, err := ctx.Auth.UserRoles(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.role_load_failed")
			return
		}
		// Role names are preset data: the row keeps the source-language text,
		// and sys_translation carries the other languages under role.<code>.
		// Without this the avatar menu showed 超级管理员 in an English UI.
		roles = localizePreset(r, ctx.Auth, roles,
			func(role store.UserRole) []string {
				return []string{"role." + role.Code}
			},
			func(role *store.UserRole, values map[string]string) {
				if s, ok := values["role."+role.Code]; ok {
					role.Name = s
				}
			})
		roleCodes := make([]string, 0, len(roles))
		roleNames := make([]string, 0, len(roles))
		for _, role := range roles {
			roleCodes = append(roleCodes, role.Code)
			roleNames = append(roleNames, role.Name)
		}
		permissions, err := ctx.Auth.PermissionCodes(r.Context(), userID, user.IsSuperAdmin)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.permission_load_failed")
			return
		}
		department, err := ctx.Auth.DepartmentRef(r.Context(), user.DepartmentID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.department_load_failed")
			return
		}
		// The department name is preset data like the role names above: the
		// stored text is the source, sys_translation carries the rest.
		department = localizeRef(r, ctx.Auth, "dept.", department)
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_policy_load_failed")
			return
		}
		ok(w, r, profileResponse{
			ID: user.ID, UserID: strconv.FormatUint(user.ID, 10),
			Username: user.Username, RealName: user.DisplayName, Email: user.Email,
			Mobile: user.Mobile, DepartmentName: department.Name,
			Roles: roleCodes, RoleNames: roleNames, Permissions: permissions, HomePath: "/analytics",
			IsSuperAdmin: user.IsSuperAdmin, MustChangePassword: user.MustChangePassword, PasswordMinLength: settings.PasswordMinLength,
		})
	}
}

type updateProfileRequest struct {
	RealName string `json:"realName"`
	Email    string `json:"email"`
	Mobile   string `json:"mobile"`
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func updateProfileHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
			return
		}
		var req updateProfileRequest
		if !decode(w, r, &req) {
			return
		}
		req.RealName = strings.TrimSpace(req.RealName)
		req.Email = strings.TrimSpace(req.Email)
		req.Mobile = strings.TrimSpace(req.Mobile)
		if req.RealName == "" || len(req.RealName) > 128 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.profile_display_name_invalid")
			return
		}
		if validation.ValidateEmail(req.Email) != nil {
			fail(w, r, http.StatusBadRequest, 40001, "errors.email_invalid")
			return
		}
		if validation.ValidateMobile(req.Mobile) != nil {
			fail(w, r, http.StatusBadRequest, 40001, "errors.mobile_invalid")
			return
		}
		if err := ctx.Auth.UpdateSelfProfile(r.Context(), userID, req.RealName, req.Email, req.Mobile); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.user_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func changePasswordHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
			return
		}
		var req changePasswordRequest
		if !decode(w, r, &req) {
			return
		}
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_policy_load_failed")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil || user.Status != 1 {
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		if validation.ValidatePassword(req.NewPassword, user.Username, settings.PasswordMinLength) != nil {
			fail(w, r, http.StatusBadRequest, 40002, "errors.password_invalid")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)) != nil {
			fail(w, r, http.StatusBadRequest, 40003, "errors.password_incorrect")
			return
		}
		if req.OldPassword == req.NewPassword {
			fail(w, r, http.StatusBadRequest, 40004, "errors.password_same")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_process_failed")
			return
		}
		if err := ctx.Auth.ChangeSelfPassword(r.Context(), userID, string(hash)); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.user_not_found")
			return
		}
		clearRefreshCookie(w, ctx)
		ok(w, r, map[string]bool{"success": true, "reloginRequired": true})
	}
}

func permissionCodesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		codes, err := ctx.Auth.PermissionCodes(r.Context(), userID, user.IsSuperAdmin)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.permission_load_failed")
			return
		}
		ok(w, r, codes)
	}
}

func menusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := currentUserID(r.Context())
		if !valid {
			fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, 40105, "errors.account_unavailable")
			return
		}
		menus, err := ctx.Auth.Menus(r.Context(), userID, user.IsSuperAdmin)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.menu_load_failed")
			return
		}
		menus = localizePreset(r, ctx.Auth, menus,
			func(m store.Menu) []string { return []string{"menu." + m.Code} },
			func(m *store.Menu, v map[string]string) {
				if s, ok := v["menu."+m.Code]; ok {
					m.Name = s
				}
			})
		ok(w, r, buildRouteTree(menus, user.IsSuperAdmin))
	}
}

func pairResponse(ctx *app.Context, pair security.Pair) tokenResponse {
	return tokenResponse{
		AccessToken: pair.AccessToken, TokenType: "Bearer",
		ExpiresIn: ctx.Config.AccessExpire, RefreshExpiresIn: ctx.Config.RefreshExpire,
	}
}

func writeRefreshCookie(w http.ResponseWriter, ctx *app.Context, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: ctx.Config.RefreshCookieName, Value: token, Path: "/api/auth",
		Domain: ctx.Config.CookieDomain, MaxAge: int(ctx.Config.RefreshExpire),
		HttpOnly: true, Secure: ctx.Config.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter, ctx *app.Context) {
	http.SetCookie(w, &http.Cookie{
		Name: ctx.Config.RefreshCookieName, Value: "", Path: "/api/auth",
		Domain: ctx.Config.CookieDomain, MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: ctx.Config.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func buildRouteTree(menus []store.Menu, superAdmin bool) []*routeNode {
	nodes := make(map[uint64]*routeNode, len(menus))
	ordered := make([]*routeNode, 0, len(menus))
	for _, menu := range menus {
		hideInMenu := !menu.Visible
		if superAdmin {
			hideInMenu = false
		}
		meta := routeMeta{
			Title: menu.Name, Icon: menu.Icon, Order: menu.Sort, HideInMenu: hideInMenu,
			KeepAlive: menu.KeepAlive, AffixTab: menu.AffixTab, Link: menu.ExternalURL,
		}
		// Affixed tabs (e.g. 工作台) must stay open: hide close/unpin controls.
		if menu.AffixTab {
			closable := false
			meta.TabClosable = &closable
		}
		node := &routeNode{
			ID: menu.ID, ParentID: menu.ParentID, Path: menu.RoutePath, Name: menu.RouteName,
			Component: menu.Component, Redirect: menu.Redirect, Meta: meta,
		}
		nodes[node.ID] = node
		ordered = append(ordered, node)
	}
	root := make([]*routeNode, 0)
	for _, node := range ordered {
		parent, exists := nodes[node.ParentID]
		if node.ParentID == 0 || !exists {
			root = append(root, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	for _, node := range ordered {
		if node.Redirect == "" && len(node.Children) > 0 {
			node.Redirect = node.Children[0].Path
		}
		// Directory routes are mounted under Root BasicLayout; keep layout marker only for leaf-less nodes.
		if len(node.Children) > 0 && node.Component == "BasicLayout" {
			node.Component = ""
		}
	}
	for _, node := range root {
		keepParentVisibleForChildren(node)
	}
	return root
}

func keepParentVisibleForChildren(node *routeNode) bool {
	hasVisibleChild := false
	for _, child := range node.Children {
		if keepParentVisibleForChildren(child) {
			hasVisibleChild = true
		}
	}
	if hasVisibleChild {
		node.Meta.HideInMenu = false
	}
	return !node.Meta.HideInMenu
}

func currentUserID(ctx context.Context) (uint64, bool) {
	value := ctx.Value(ctxkey.UserID)
	switch typed := value.(type) {
	case float64:
		return uint64(typed), typed > 0
	case int64:
		return uint64(typed), typed > 0
	case uint64:
		return typed, typed > 0
	case string:
		parsed, err := strconv.ParseUint(typed, 10, 64)
		return parsed, err == nil && parsed > 0
	default:
		return 0, false
	}
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
