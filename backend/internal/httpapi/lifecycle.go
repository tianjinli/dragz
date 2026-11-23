package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/internal/validation"
)

// This file is the tenant self-service surface: signup, invitations and
// membership management. The platform staff directory lives at the bottom.

// signupEnabledFeature is the switch that turns public registration on or off.
// It defaults to off so a deployment that never configures it does not expose
// an open registration endpoint by accident.
const signupEnabledFeature = "feature.signup.enabled"

// signupPlanConfig points at the plan a new tenant starts on. Missing means
// "the lowest-sorted active plan".
const signupPlanConfig = "tenant.signup.plan"

type signupRequest struct {
	TenantName  string `json:"tenantName"`
	TenantCode  string `json:"tenantCode"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email"`
	Mobile      string `json:"mobile"`
	DisplayName string `json:"displayName"`
}

type signupStatusResponse struct {
	Enabled bool `json:"enabled"`
	// PasswordMinLength lets the form validate the same rule the server will
	// apply, instead of hardcoding a length that drifts when an operator
	// raises it in the security settings.
	PasswordMinLength int `json:"passwordMinLength"`
}

// signupStatusHandler lets the signup page hide itself when registration is
// closed, instead of showing a form that always fails.
func signupStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, err := ctx.Auth.FeatureEnabled(r.Context(), signupEnabledFeature, false)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		minLength := 0
		if settings, err := ctx.Auth.LoginSecuritySettings(r.Context()); err == nil {
			minLength = settings.PasswordMinLength
		}
		ok(w, r, signupStatusResponse{Enabled: enabled, PasswordMinLength: minLength})
	}
}

func registerHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enabled, err := ctx.Auth.FeatureEnabled(r.Context(), signupEnabledFeature, false)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		if !enabled {
			fail(w, r, http.StatusForbidden, apierrors.CodeFeatureDisabled, "errors.signup_disabled")
			return
		}
		var req signupRequest
		if !decode(w, r, &req) {
			return
		}
		req.TenantName = strings.TrimSpace(req.TenantName)
		req.TenantCode = strings.TrimSpace(req.TenantCode)
		req.Username = strings.TrimSpace(req.Username)
		req.Email = strings.TrimSpace(req.Email)
		req.Mobile = strings.TrimSpace(req.Mobile)
		req.DisplayName = strings.TrimSpace(req.DisplayName)
		if req.TenantName == "" || utf8.RuneCountInString(req.TenantName) > 128 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if req.TenantCode == "" {
			req.TenantCode = deriveTenantCode(req.TenantName)
		}
		if !validTenantCode(req.TenantCode) {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_code_invalid")
			return
		}
		if err := validation.ValidateUsername(req.Username); err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.username_invalid")
			return
		}
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.password_policy_load_failed")
			return
		}
		if err := validation.ValidatePassword(req.Password, req.Username, settings.PasswordMinLength); err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.password_invalid")
			return
		}
		if err := validation.ValidateEmail(req.Email); err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.email_invalid")
			return
		}
		if err := validation.ValidateMobile(req.Mobile); err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.mobile_invalid")
			return
		}
		planCode, err := ctx.Auth.ConfigValue(r.Context(), signupPlanConfig)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.password_process_failed")
			return
		}
		result, err := ctx.Tenants.SignupTenant(r.Context(), store.SignupInput{
			TenantCode: req.TenantCode, TenantName: req.TenantName,
			Username: req.Username, PasswordHash: string(hash),
			Email: req.Email, Mobile: req.Mobile, DisplayName: req.DisplayName,
			PlanCode: planCode,
		})
		if err != nil {
			writeSignupError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusCreated)
		ok(w, r, result)
	}
}

// writeSignupError turns a provisioning failure into a message the signup form
// can show. A duplicate is attributed to the column that actually collided.
func writeSignupError(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, store.ErrDuplicateEntry) {
		fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
		return
	}
	switch store.ConstraintName(err) {
	case "uk_user_username", "uk_user_username_live":
		fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.username_exists")
	case "uk_tenant_code":
		fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.tenant_code_exists")
	case "uk_tenant_slug":
		fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.tenant_code_exists")
	default:
		fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.duplicate")
	}
}

// deriveTenantCode builds a code from a display name. A name with no ASCII
// letters (for example a Chinese company name) falls back to a random suffix,
// because a code has to be a URL-safe identifier.
func deriveTenantCode(name string) string {
	code := store.SlugFromCode(name)
	if len(code) >= 3 {
		return code
	}
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "tenant"
	}
	return "tenant-" + hex.EncodeToString(buf)
}

type invitePreviewResponse struct {
	TenantName string     `json:"tenantName"`
	Email      string     `json:"email"`
	Status     string     `json:"status"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
}

// invitePreviewHandler resolves an invite link before the visitor has an
// account. It returns the tenant name so the page can say what is being
// joined, and nothing that would help guessing a token.
func invitePreviewHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invite, err := ctx.Tenants.InviteByToken(r.Context(), pathvarValue(r, "token"))
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.invite_invalid")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		if invite.Status == store.InvitePending && !invite.ExpiresAt.After(time.Now()) {
			invite.Status = store.InviteExpired
		}
		ok(w, r, invitePreviewResponse{
			TenantName: invite.TenantName, Email: invite.Email,
			Status: invite.Status, ExpiresAt: invite.ExpiresAt, AcceptedAt: invite.AcceptedAt,
		})
	}
}

type acceptInviteResponse struct {
	TenantID   uint64 `json:"tenantId"`
	TenantName string `json:"tenantName"`
}

// acceptInviteHandler joins the signed-in account to the invite's tenant. The
// client follows up with /api/auth/switch-tenant when the account already had
// another tenant open.
func acceptInviteHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, okUser := currentUserID(r.Context())
		if !okUser {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.unauthorized")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.unauthorized")
			return
		}
		invite, err := ctx.Tenants.AcceptInvite(r.Context(), pathvarValue(r, "token"), userID, user.Email)
		switch {
		case errors.Is(err, store.ErrInviteEmailMismatch):
			fail(w, r, http.StatusForbidden, apierrors.CodeForbidden, "errors.invite_email_mismatch")
			return
		case errors.Is(err, store.ErrInviteInvalid), errors.Is(err, store.ErrNotFound):
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.invite_invalid")
			return
		case errors.Is(err, store.ErrQuotaExceeded):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.quota_exceeded")
			return
		case err != nil:
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, acceptInviteResponse{TenantID: invite.TenantID, TenantName: invite.TenantName})
	}
}

// ---------------------------------------------------------------------------
// Membership
// ---------------------------------------------------------------------------

type createInviteRequest struct {
	Email         string  `json:"email"`
	RoleIDs       []int64 `json:"roleIds"`
	DepartmentID  *uint64 `json:"departmentId"`
	IsTenantAdmin bool    `json:"isTenantAdmin"`
	TTLHours      int     `json:"ttlHours"`
}

type createInviteResponse struct {
	Invite store.Invite `json:"invite"`
	// Token is returned exactly once. Only its hash is stored.
	Token string `json:"token"`
}

func createInviteHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		userID, okUser := currentUserID(r.Context())
		if !okTenant || tenantID == 0 || !okUser {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req createInviteRequest
		if !decode(w, r, &req) {
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		if err := validation.ValidateEmail(req.Email); err != nil {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.email_invalid")
			return
		}
		if req.TTLHours < 0 || req.TTLHours > 24*30 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		ttl := time.Duration(req.TTLHours) * time.Hour
		invite, token, err := ctx.Tenants.CreateInvite(r.Context(), store.InviteInput{
			TenantID: tenantID, Email: req.Email, RoleIDs: req.RoleIDs,
			DepartmentID: req.DepartmentID, TenantAdmin: req.IsTenantAdmin,
			InvitedBy: userID, TTL: ttl,
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		w.WriteHeader(http.StatusCreated)
		ok(w, r, createInviteResponse{Invite: invite, Token: token})
	}
}

func listInvitesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		if !okTenant || tenantID == 0 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		items, err := ctx.Tenants.ListInvites(r.Context(), tenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

func deleteInviteHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		id, okID := pathID(r)
		if !okTenant || tenantID == 0 || !okID {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		err := ctx.Tenants.RevokeInvite(r.Context(), tenantID, id)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.invite_invalid")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listMembersHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		if !okTenant || tenantID == 0 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		items, err := ctx.Tenants.ListMembers(r.Context(), tenantID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

type memberUpdateRequest struct {
	Status        *int16  `json:"status"`
	IsTenantAdmin *bool   `json:"isTenantAdmin"`
	DepartmentID  *uint64 `json:"departmentId"`
	RoleIDs       []int64 `json:"roleIds"`
}

func updateMemberHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		userID, okID := pathID(r)
		if !okTenant || tenantID == 0 || !okID {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		var req memberUpdateRequest
		if !decode(w, r, &req) {
			return
		}
		if req.Status != nil && *req.Status != 0 && *req.Status != 1 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		err := ctx.Tenants.UpdateMember(r.Context(), tenantID, userID, store.MemberUpdate{
			Status: req.Status, IsTenantAdmin: req.IsTenantAdmin,
			DepartmentID: req.DepartmentID, RoleIDs: req.RoleIDs,
		})
		switch {
		case errors.Is(err, store.ErrLastTenantAdmin):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.member_last_admin")
			return
		case errors.Is(err, store.ErrNotFound):
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.member_not_found")
			return
		case err != nil:
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func removeMemberHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, okTenant := ctxkey.TenantIDFrom(r.Context())
		userID, okID := pathID(r)
		if !okTenant || tenantID == 0 || !okID {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		err := ctx.Tenants.RemoveMember(r.Context(), tenantID, userID)
		switch {
		case errors.Is(err, store.ErrLastTenantAdmin):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.member_last_admin")
			return
		case errors.Is(err, store.ErrNotFound):
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.member_not_found")
			return
		case err != nil:
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

// ---------------------------------------------------------------------------
// Platform staff directory
// ---------------------------------------------------------------------------

type grantOperatorRequest struct {
	UserID uint64 `json:"userId"`
	Note   string `json:"note"`
}

func listPlatformOperatorsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Tenants.ListPlatformOperators(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

func grantPlatformOperatorHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req grantOperatorRequest
		if !decode(w, r, &req) {
			return
		}
		actor, _ := currentUserID(r.Context())
		err := ctx.Tenants.GrantPlatformOperator(r.Context(), req.UserID, actor, req.Note)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.user_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func revokePlatformOperatorHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		err := ctx.Tenants.RevokePlatformOperator(r.Context(), userID)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.operator_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}
