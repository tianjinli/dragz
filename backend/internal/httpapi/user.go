package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest/pathvar"
	"golang.org/x/crypto/bcrypt"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/httpapi/logic"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/internal/validation"
)

type saveUserRequest struct {
	DepartmentID       *uint64  `json:"departmentId"`
	Username           string   `json:"username"`
	Password           string   `json:"password"`
	DisplayName        string   `json:"displayName"`
	Email              string   `json:"email"`
	Mobile             string   `json:"mobile"`
	Gender             int8     `json:"gender"`
	Status             int8     `json:"status"`
	MustChangePassword bool     `json:"mustChangePassword"`
	Remark             string   `json:"remark"`
	RoleIDs            []uint64 `json:"roleIds"`
	PositionIDs        []uint64 `json:"positionIds"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

// localizeUserRows renders the department, role and position names of user rows
// in the request language. Those three are preset data — the translation page
// already holds their English — but the list query aggregates them into single
// strings, so the codes travel alongside and are what the lookups are keyed by.
//
// Everything else on a user row (username, display name, email, mobile) is the
// administrator's own data: it is stored exactly as typed and is never
// translated, because a translated account name would not match anything the
// operator searches for.
func localizeUserRows(r *http.Request, loader translationLoader, items []store.UserListItem) []store.UserListItem {
	lang := pkgi18n.Lang(r)
	if lang == pkgi18n.SourceLanguage || len(items) == 0 {
		return items
	}
	seen := make(map[string]bool, len(items)*3)
	keys := make([]string, 0, len(items)*3)
	add := func(key string) {
		if key != "" && !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, item := range items {
		if item.DepartmentCode != "" {
			add("dept." + item.DepartmentCode)
		}
		for _, ref := range item.Roles {
			add("role." + ref.Code)
		}
		for _, ref := range item.Positions {
			add("post." + ref.Code)
		}
	}
	if len(keys) == 0 {
		return items
	}
	values, err := loader.TranslationsByKey(r.Context(), lang, keys)
	if err != nil || len(values) == 0 {
		return items
	}
	for i := range items {
		if s, ok := values["dept."+items[i].DepartmentCode]; ok {
			items[i].DepartmentName = s
		}
		items[i].RoleNames = joinLocalizedNames(values, "role.", items[i].Roles, items[i].RoleNames)
		items[i].PositionNames = joinLocalizedNames(values, "post.", items[i].Positions, items[i].PositionNames)
	}
	return items
}

// joinLocalizedNames rebuilds the comma-separated label the list query
// aggregates, swapping in the translated name where one exists and keeping the
// source text where it does not. Without refs (an old row, or a parse failure)
// the aggregated text is returned untouched rather than blanked.
func joinLocalizedNames(values map[string]string, prefix string, refs []store.NamedRef, fallback string) string {
	if len(refs) == 0 {
		return fallback
	}
	names := make([]string, len(refs))
	for i, ref := range refs {
		if s, ok := values[prefix+ref.Code]; ok {
			names[i] = s
			continue
		}
		names[i] = ref.Name
	}
	return strings.Join(names, ", ")
}

func listUsersHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page := queryInt(query.Get("page"), 1)
		pageSize := queryInt(query.Get("pageSize"), 20)
		departmentID := queryUint64(query.Get("departmentId"))
		var status *int8
		if value := strings.TrimSpace(query.Get("status")); value != "" {
			parsed, err := strconv.ParseInt(value, 10, 8)
			if err != nil {
				fail(w, r, http.StatusBadRequest, 40001, "errors.user_status_invalid")
				return
			}
			converted := int8(parsed)
			status = &converted
		}
		scope, valid := requestUserDataScope(ctx, w, r)
		if !valid {
			return
		}
		items, total, err := logic.NewUserLogic(logicContext(ctx)).ListUsers(r.Context(), store.UserListFilter{
			Keyword: query.Get("keyword"), DepartmentID: departmentID, Status: status,
			Page: page, PageSize: pageSize, Scope: &scope,
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.user_list_failed")
			return
		}
		items = localizeUserRows(r, ctx.Auth, items)
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func getUserHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_id_invalid")
			return
		}
		if !requestCanAccessUser(ctx, w, r, userID) {
			return
		}
		detail, err := logic.NewUserLogic(logicContext(ctx)).UserDetail(r.Context(), userID)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.user_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.user_detail_failed")
			return
		}
		// One row, same preset-name lookup as the list.
		detail.UserListItem = localizeUserRows(
			r, ctx.Auth, []store.UserListItem{detail.UserListItem})[0]
		ok(w, r, detail)
	}
}

func createUserHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveUserRequest
		if !decode(w, r, &req) {
			return
		}
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_policy_load_failed")
			return
		}
		if !validateSaveUser(w, r, req, true, settings.PasswordMinLength) {
			return
		}
		scope, valid := requestUserDataScope(ctx, w, r)
		if !valid {
			return
		}
		if !scope.All && (req.DepartmentID == nil || !scope.AllowsDepartment(*req.DepartmentID)) {
			fail(w, r, http.StatusForbidden, 40302, "errors.user_create_scope")
			return
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_process_failed")
			return
		}
		userID, err := logic.NewUserLogic(logicContext(ctx)).CreateUser(r.Context(), saveUserInput(req, string(passwordHash)))
		if err != nil {
			writeSaveUserError(w, r, err)
			return
		}
		ok(w, r, map[string]uint64{"id": userID})
	}
}

func updateUserHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_id_invalid")
			return
		}
		if !requestCanAccessUser(ctx, w, r, userID) {
			return
		}
		var req saveUserRequest
		if !decode(w, r, &req) || !validateSaveUser(w, r, req, false, 0) {
			return
		}
		scope, valid := requestUserDataScope(ctx, w, r)
		if !valid {
			return
		}
		if !scope.All && req.DepartmentID != nil && !scope.AllowsDepartment(*req.DepartmentID) {
			fail(w, r, http.StatusForbidden, 40302, "errors.user_move_scope")
			return
		}
		if err := logic.NewUserLogic(logicContext(ctx)).UpdateUser(r.Context(), userID, saveUserInput(req, "")); err != nil {
			writeSaveUserError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updateUserStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_id_invalid")
			return
		}
		if !requestCanAccessUser(ctx, w, r, userID) {
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_status_enum")
			return
		}
		if err := logic.NewUserLogic(logicContext(ctx)).UpdateUserStatus(r.Context(), userID, req.Status); err != nil {
			writeSaveUserError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteUserHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_id_invalid")
			return
		}
		if !requestCanAccessUser(ctx, w, r, userID) {
			return
		}
		currentID, _ := currentUserID(r.Context())
		if currentID == userID {
			fail(w, r, http.StatusConflict, 40901, "errors.user_delete_self")
			return
		}
		if err := logic.NewUserLogic(logicContext(ctx)).DeleteUser(r.Context(), userID); err != nil {
			fail(w, r, http.StatusConflict, 40902, "errors.user_delete_protected")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func resetUserPasswordHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_id_invalid")
			return
		}
		if !requestCanAccessUser(ctx, w, r, userID) {
			return
		}
		var req resetPasswordRequest
		if !decode(w, r, &req) {
			return
		}
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_policy_load_failed")
			return
		}
		user, err := ctx.Auth.FindUserByID(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.user_not_found")
			return
		}
		if validation.ValidatePassword(req.Password, user.Username, settings.PasswordMinLength) != nil {
			fail(w, r, http.StatusBadRequest, 40002, "errors.password_invalid")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.password_process_failed")
			return
		}
		if err := logic.NewUserLogic(logicContext(ctx)).ResetUserPassword(r.Context(), userID, string(hash)); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.user_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func forceLogoutUserHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.user_id_invalid")
			return
		}
		if !requestCanAccessUser(ctx, w, r, userID) {
			return
		}
		if err := ctx.Auth.ForceLogoutUser(r.Context(), userID); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.user_kick_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func requestUserDataScope(ctx *app.Context, w http.ResponseWriter, r *http.Request) (store.DataScope, bool) {
	operatorID, valid := currentUserID(r.Context())
	if !valid {
		fail(w, r, http.StatusUnauthorized, 40100, "errors.invalid_session")
		return store.DataScope{}, false
	}
	scope, err := ctx.Auth.ResolveUserDataScope(r.Context(), operatorID)
	if err != nil {
		fail(w, r, http.StatusInternalServerError, 50000, "errors.data_scope_resolve_failed")
		return store.DataScope{}, false
	}
	return scope, true
}

func requestCanAccessUser(ctx *app.Context, w http.ResponseWriter, r *http.Request, targetUserID uint64) bool {
	scope, valid := requestUserDataScope(ctx, w, r)
	if !valid {
		return false
	}
	allowed, err := ctx.Auth.DataScopeAllowsUser(r.Context(), scope, targetUserID)
	if errors.Is(err, sql.ErrNoRows) {
		fail(w, r, http.StatusNotFound, 40401, "errors.user_not_found")
		return false
	}
	if err != nil {
		fail(w, r, http.StatusInternalServerError, 50000, "errors.data_scope_check_failed")
		return false
	}
	if !allowed {
		fail(w, r, http.StatusForbidden, 40302, "errors.user_out_of_scope")
		return false
	}
	return true
}

func validateSaveUser(w http.ResponseWriter, r *http.Request, req saveUserRequest, create bool, passwordMinLength int) bool {
	if create && validation.ValidateUsername(req.Username) != nil {
		fail(w, r, http.StatusBadRequest, 40001, "errors.username_invalid")
		return false
	}
	if strings.TrimSpace(req.DisplayName) == "" || len(req.DisplayName) > 128 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.user_display_name_invalid")
		return false
	}
	if create && validation.ValidatePassword(req.Password, req.Username, passwordMinLength) != nil {
		fail(w, r, http.StatusBadRequest, 40002, "errors.password_invalid")
		return false
	}
	if validation.ValidateEmail(strings.TrimSpace(req.Email)) != nil {
		fail(w, r, http.StatusBadRequest, 40001, "errors.email_invalid")
		return false
	}
	if validation.ValidateMobile(strings.TrimSpace(req.Mobile)) != nil {
		fail(w, r, http.StatusBadRequest, 40001, "errors.mobile_invalid")
		return false
	}
	if req.Status != 0 && req.Status != 1 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.user_status_enum")
		return false
	}
	return true
}

func saveUserInput(req saveUserRequest, passwordHash string) store.SaveUserInput {
	return store.SaveUserInput{
		DepartmentID: req.DepartmentID, Username: strings.TrimSpace(req.Username), PasswordHash: passwordHash,
		DisplayName: strings.TrimSpace(req.DisplayName), Email: strings.TrimSpace(req.Email),
		Mobile: strings.TrimSpace(req.Mobile), Gender: req.Gender, Status: req.Status,
		MustChangePassword: req.MustChangePassword, Remark: strings.TrimSpace(req.Remark),
		RoleIDs: req.RoleIDs, PositionIDs: req.PositionIDs,
	}
}

func writeSaveUserError(w http.ResponseWriter, r *http.Request, err error) {
	failHTTP(w, r, apierrors.MapUserSave(err))
}

func pathID(r *http.Request) (uint64, bool) {
	value := pathvar.Vars(r)["id"]
	parsed, err := strconv.ParseUint(value, 10, 64)
	return parsed, err == nil && parsed > 0
}

func queryInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func queryUint64(value string) uint64 {
	parsed, _ := strconv.ParseUint(value, 10, 64)
	return parsed
}
