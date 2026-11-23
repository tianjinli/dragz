package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

func requirePlatform(appCtx *app.Context, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !platformCaller(r, appCtx) {
			fail(w, r, http.StatusForbidden, apierrors.CodeForbidden, "errors.platform_forbidden")
			return
		}
		next(w, r.WithContext(ctxkey.WithRLSBypass(r.Context(), true)))
	}
}

func platformCaller(r *http.Request, appCtx *app.Context) bool {
	if ctxkey.PlatformOperatorFrom(r.Context()) {
		return true
	}
	userID, ok := currentUserID(r.Context())
	if !ok || appCtx == nil || appCtx.Auth == nil {
		return false
	}
	user, err := appCtx.Auth.FindUserByID(r.Context(), userID)
	return err == nil && user.Status == 1 && user.IsSuperAdmin
}

type createPlatformTenantRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type platformStatusRequest struct {
	Status int16 `json:"status"`
}

type platformSubscriptionRequest struct {
	PlanID uint64 `json:"planId"`
	Status string `json:"status"`
}

type platformImpersonateRequest struct {
	Reason string `json:"reason"`
	Hours  int    `json:"hours"`
}

func listPlatformTenantsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		items, err := ctx.Tenants.ListPlatformTenants(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

func createPlatformTenantHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createPlatformTenantRequest
		if !decode(w, r, &req) {
			return
		}
		req.Code = strings.TrimSpace(req.Code)
		req.Name = strings.TrimSpace(req.Name)
		if !validTenantCode(req.Code) || req.Name == "" || utf8.RuneCountInString(req.Name) > 128 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		slug := strings.ToLower(strings.TrimSpace(req.Slug))
		if slug == "" {
			slug = store.SlugFromCode(req.Code)
		} else if message := slugErrorKey(slug); message != "" {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, message)
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		item, err := ctx.Tenants.CreatePlatformTenant(r.Context(), req.Code, req.Name, slug)
		if writeTenantMutationError(w, r, err) {
			return
		}
		ok(w, r, item)
	}
}

func updatePlatformTenantStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req platformStatusRequest
		if !decode(w, r, &req) {
			return
		}
		if req.Status != 0 && req.Status != 1 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.UpdatePlatformTenantStatus(r.Context(), id, req.Status)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listPlatformPlansHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		items, err := ctx.Tenants.ListPlatformPlans(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, items)
	}
}

func updatePlatformSubscriptionHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req platformSubscriptionRequest
		if !decode(w, r, &req) {
			return
		}
		req.Status = strings.TrimSpace(req.Status)
		if req.Status == "" {
			req.Status = "active"
		}
		if req.PlanID == 0 || utf8.RuneCountInString(req.Status) > 32 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		subscriptionID, err := ctx.Tenants.ReplaceTenantSubscription(r.Context(), id, req.PlanID, req.Status)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.plan_not_found")
			return
		}
		if errors.Is(err, store.ErrInvalidSubscriptionTransition) {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.subscription_transition")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]any{"id": subscriptionID, "tenantId": id, "planId": req.PlanID, "status": req.Status})
	}
}

func impersonatePlatformTenantHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.tenant_invalid")
			return
		}
		var req platformImpersonateRequest
		if !decode(w, r, &req) {
			return
		}
		req.Reason = strings.TrimSpace(req.Reason)
		if req.Reason == "" || utf8.RuneCountInString(req.Reason) > 500 || req.Hours < 1 || req.Hours > 24 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		userID, okUser := currentUserID(r.Context())
		if !okUser || ctx.Tenants == nil || ctx.Tokens == nil || ctx.Auth == nil {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.invalid_session")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil || user.Status != 1 {
			fail(w, r, http.StatusUnauthorized, apierrors.CodeUnauthorized, "errors.account_unavailable")
			return
		}
		expiresAt := time.Now().Add(time.Duration(req.Hours) * time.Hour)
		impersonationID, err := ctx.Tenants.InsertImpersonation(r.Context(), id, userID, req.Reason, expiresAt)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.tenant_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		accessToken, accessExpires, err := ctx.Tokens.IssueAccess(user.ID, user.Username, user.TokenVersion, id, time.Duration(req.Hours)*time.Hour)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.token_issue_failed")
			return
		}
		ok(w, r, map[string]any{
			"accessToken":     accessToken,
			"tokenType":       "Bearer",
			"expiresIn":       int64(time.Until(accessExpires).Seconds()),
			"impersonationId": impersonationID,
			"tenantId":        id,
		})
	}
}

func validTenantCode(code string) bool {
	if code == "" || len(code) > 64 {
		return false
	}
	for _, r := range code {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

type planEntitlementBody struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type planWriteBody struct {
	Code         string                 `json:"code"`
	Name         string                 `json:"name"`
	Description  *string                `json:"description"`
	Status       *int16                 `json:"status"`
	Sort         *int                   `json:"sort"`
	Entitlements *[]planEntitlementBody `json:"entitlements"`
}

func createPlatformPlanHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body planWriteBody
		if !decode(w, r, &body) {
			return
		}
		write, valid := normalizePlanWrite(body, true)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		item, err := ctx.Tenants.CreatePlatformPlan(r.Context(), write)
		if errors.Is(err, store.ErrDuplicateEntry) {
			fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.plan_code_taken")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, item)
	}
}

func updatePlatformPlanHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, validID := pathID(r)
		if !validID {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		var body planWriteBody
		if !decode(w, r, &body) {
			return
		}
		write, valid := normalizePlanWrite(body, false)
		if !valid {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		item, err := ctx.Tenants.UpdatePlatformPlan(r.Context(), id, write)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.plan_not_found")
			return
		}
		if errors.Is(err, store.ErrDuplicateEntry) {
			fail(w, r, http.StatusConflict, apierrors.CodeDuplicate, "errors.plan_code_taken")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, item)
	}
}

func updatePlatformPlanStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, validID := pathID(r)
		if !validID {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		var req platformStatusRequest
		if !decode(w, r, &req) {
			return
		}
		if req.Status != 0 && req.Status != 1 {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}
		if ctx.Tenants == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		err := ctx.Tenants.UpdatePlatformPlanStatus(r.Context(), id, req.Status)
		if errors.Is(err, store.ErrNotFound) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.plan_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func normalizePlanWrite(body planWriteBody, creating bool) (store.PlanWrite, bool) {
	body.Code = strings.TrimSpace(body.Code)
	body.Name = strings.TrimSpace(body.Name)
	if !validTenantCode(body.Code) || body.Name == "" || utf8.RuneCountInString(body.Name) > 128 {
		return store.PlanWrite{}, false
	}
	var description *string
	if creating || body.Description != nil {
		value := ""
		if body.Description != nil {
			value = strings.TrimSpace(*body.Description)
		}
		if utf8.RuneCountInString(value) > 500 {
			return store.PlanWrite{}, false
		}
		description = &value
	}
	if body.Status != nil && *body.Status != 0 && *body.Status != 1 {
		return store.PlanWrite{}, false
	}
	status := body.Status
	if creating && status == nil {
		enabled := int16(1)
		status = &enabled
	}
	if body.Sort != nil && (*body.Sort < -1_000_000 || *body.Sort > 1_000_000) {
		return store.PlanWrite{}, false
	}
	sort := body.Sort
	if creating && sort == nil {
		zero := 0
		sort = &zero
	}
	replace := creating || body.Entitlements != nil
	var entitlements []store.PlanEntitlement
	if replace {
		raw := []planEntitlementBody{}
		if body.Entitlements != nil {
			raw = *body.Entitlements
		}
		normalized, ok := normalizeEntitlements(raw)
		if !ok {
			return store.PlanWrite{}, false
		}
		entitlements = normalized
	}
	return store.PlanWrite{
		Code:                body.Code,
		Name:                body.Name,
		Description:         description,
		Status:              status,
		Sort:                sort,
		Entitlements:        entitlements,
		ReplaceEntitlements: replace,
	}, true
}

func normalizeEntitlements(items []planEntitlementBody) ([]store.PlanEntitlement, bool) {
	if len(items) > 100 {
		return nil, false
	}
	out := make([]store.PlanEntitlement, 0, len(items))
	index := map[string]int{}
	for _, item := range items {
		key := strings.TrimSpace(item.Key)
		value := strings.TrimSpace(item.Value)
		if !validEntitlementKey(key) || utf8.RuneCountInString(value) > 2000 {
			return nil, false
		}
		if at, ok := index[key]; ok {
			out[at].Value = value
			continue
		}
		index[key] = len(out)
		out = append(out, store.PlanEntitlement{Key: key, Value: value})
	}
	return out, true
}

func validEntitlementKey(key string) bool {
	if key == "" || len(key) > 128 {
		return false
	}
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
