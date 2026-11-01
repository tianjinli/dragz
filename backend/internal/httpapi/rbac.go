package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/httpapi/logic"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

type saveRoleRequest struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	DataScope string `json:"dataScope"`
	Sort      int    `json:"sort"`
	Status    int8   `json:"status"`
	Remark    string `json:"remark"`
}

type saveRoleGrantsRequest struct {
	DataScope     string   `json:"dataScope"`
	MenuIDs       []uint64 `json:"menuIds"`
	DepartmentIDs []uint64 `json:"departmentIds"`
}

type saveMenuRequest struct {
	ParentID    uint64 `json:"parentId"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	RouteName   string `json:"routeName"`
	RoutePath   string `json:"routePath"`
	Component   string `json:"component"`
	Redirect    string `json:"redirect"`
	Icon        string `json:"icon"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	IsSensitive bool   `json:"isSensitive"`
	Sort        int    `json:"sort"`
	Status      int8   `json:"status"`
	Visible     bool   `json:"visible"`
	KeepAlive   bool   `json:"keepAlive"`
	AffixTab    bool   `json:"affixTab"`
	ExternalURL string `json:"externalUrl"`
}

func listRolesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, valid := optionalStatus(r.URL.Query().Get("status"))
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_status_invalid")
			return
		}
		items, err := logic.NewRBACLogic(logicContext(ctx)).ListRoles(r.Context(), r.URL.Query().Get("keyword"), status)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.role_list_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(role store.Role) []string {
				return []string{"role." + role.Code, "role." + role.Code + ".remark"}
			},
			func(role *store.Role, v map[string]string) {
				if s, ok := v["role."+role.Code]; ok {
					role.Name = s
				}
				if s, ok := v["role."+role.Code+".remark"]; ok {
					role.Remark = s
				}
			})
		ok(w, r, items)
	}
}

func createRoleHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveRoleRequest
		if !decode(w, r, &req) || !validateRole(w, r, req) {
			return
		}
		input := roleInput(req)
		save, err := preparePresetSave(ctx.Auth, r, rolePresetEdit(0, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		id, err := logic.NewRBACLogic(logicContext(ctx)).CreateRole(r.Context(), input)
		if err != nil {
			writeRBACError(w, r, err, "subjects.role")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateRoleHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_id_invalid")
			return
		}
		var req saveRoleRequest
		if !decode(w, r, &req) || !validateRole(w, r, req) {
			return
		}
		input := roleInput(req)
		save, err := preparePresetSave(ctx.Auth, r, rolePresetEdit(id, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		// A save made in another language rewrites the translation, not the
		// Chinese columns.
		input.Name, input.Remark = save.SourceName(), save.SourceRemark()
		if err := logic.NewRBACLogic(logicContext(ctx)).UpdateRole(r.Context(), id, input); err != nil {
			writeRBACError(w, r, err, "subjects.role")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

// rolePresetEdit describes a role save's translatable texts: name and remark,
// both keyed by the role code.
func rolePresetEdit(id uint64, input store.SaveRoleInput) presetTextEdit {
	return presetTextEdit{
		Namespace: "role", Prefix: "role", ID: id,
		Code: input.Code, Name: input.Name, Remark: input.Remark, HasRemark: true,
	}
}

func deleteRoleHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_id_invalid")
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).DeleteRole(r.Context(), id); err != nil {
			writeRBACError(w, r, err, "subjects.role")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updateRoleStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_id_invalid")
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_status_enum")
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).UpdateRoleStatus(r.Context(), id, req.Status); err != nil {
			writeRBACError(w, r, err, "subjects.role")
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).AfterRoleGrantsChanged(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func getRoleGrantsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_id_invalid")
			return
		}
		grants, err := logic.NewRBACLogic(logicContext(ctx)).GetRoleGrants(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.role_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.role_perms_load_failed")
			return
		}
		ok(w, r, grants)
	}
}

func saveRoleGrantsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.role_id_invalid")
			return
		}
		var req saveRoleGrantsRequest
		if !decode(w, r, &req) || !validDataScope(req.DataScope) {
			if req.DataScope != "" {
				fail(w, r, http.StatusBadRequest, 40001, "errors.data_scope_invalid")
			}
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).SaveRoleGrants(r.Context(), store.RoleGrants{
			RoleID: id, DataScope: req.DataScope, MenuIDs: req.MenuIDs,
			DepartmentIDs: req.DepartmentIDs,
		}); err != nil {
			writeRBACError(w, r, err, "subjects.role")
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).AfterRoleGrantsChanged(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.internal")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listMenuResourcesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := logic.NewRBACLogic(logicContext(ctx)).ListMenuResources(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.menu_list_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(m store.MenuResource) []string {
				if m.Type == "button" {
					return []string{"perm." + m.Code}
				}
				return []string{"menu." + m.Code}
			},
			func(m *store.MenuResource, v map[string]string) {
				if m.Type == "button" {
					if s, ok := v["perm."+m.Code]; ok {
						m.Name = s
					}
					return
				}
				if s, ok := v["menu."+m.Code]; ok {
					m.Name = s
				}
			})
		ok(w, r, items)
	}
}

func createMenuHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveMenuRequest
		if !decode(w, r, &req) || !validateMenu(w, r, req) {
			return
		}
		input := menuInput(req)
		save, err := preparePresetSave(ctx.Auth, r, menuPresetEdit(0, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		id, err := logic.NewRBACLogic(logicContext(ctx)).CreateMenu(r.Context(), input)
		if err != nil {
			writeRBACError(w, r, err, "subjects.menu")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateMenuHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.menu_id_invalid")
			return
		}
		var req saveMenuRequest
		if !decode(w, r, &req) || !validateMenu(w, r, req) {
			return
		}
		input := menuInput(req)
		save, err := preparePresetSave(ctx.Auth, r, menuPresetEdit(id, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		input.Name = save.SourceName()
		if err := logic.NewRBACLogic(logicContext(ctx)).UpdateMenu(r.Context(), id, input); err != nil {
			writeRBACError(w, r, err, "subjects.menu")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

// menuPresetEdit describes a menu save. Permission buttons are sys_menu rows
// like pages are, but they are translated under perm.<code>.
func menuPresetEdit(id uint64, input store.SaveMenuInput) presetTextEdit {
	prefix := "menu"
	if input.Type == "button" {
		prefix = "perm"
	}
	return presetTextEdit{
		Namespace: "menu", Prefix: prefix, ID: id, Code: input.Code, Name: input.Name,
	}
}

func deleteMenuHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.menu_id_invalid")
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).DeleteMenu(r.Context(), id); err != nil {
			writeRBACError(w, r, err, "subjects.menu")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updateMenuStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.menu_id_invalid")
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.menu_status_invalid")
			return
		}
		if err := logic.NewRBACLogic(logicContext(ctx)).UpdateMenuStatus(r.Context(), id, req.Status); err != nil {
			writeRBACError(w, r, err, "subjects.menu")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func validateRole(w http.ResponseWriter, r *http.Request, req saveRoleRequest) bool {
	if strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Name) == "" || !validDataScope(req.DataScope) {
		fail(w, r, http.StatusBadRequest, 40001, "errors.role_fields_invalid")
		return false
	}
	if req.Status != 0 && req.Status != 1 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.role_status_enum")
		return false
	}
	return true
}

func validDataScope(value string) bool {
	switch value {
	case "self", "department", "department_and_children", "custom", "all":
		return true
	default:
		return false
	}
}

func roleInput(req saveRoleRequest) store.SaveRoleInput {
	return store.SaveRoleInput{
		Code: strings.TrimSpace(req.Code), Name: strings.TrimSpace(req.Name), DataScope: req.DataScope,
		Sort: req.Sort, Status: req.Status, Remark: strings.TrimSpace(req.Remark),
	}
}

func validateMenu(w http.ResponseWriter, r *http.Request, req saveMenuRequest) bool {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Code) == "" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.menu_fields_invalid")
		return false
	}
	// Keep accepting "button": the startup sync owns those rows, but the admin
	// UI still reads and renames them.
	if !isAllowedMenuType(req.Type) {
		fail(w, r, http.StatusBadRequest, 40001, "errors.menu_type_invalid")
		return false
	}
	// A colon means "permission code", which only button rows may use. This is
	// what keeps a page code from colliding with a permission code, since
	// sys_menu.code is unique across both. The rule cuts both ways: a button
	// without a colon would sit in the page namespace and shadow a route.
	if req.Type == "button" {
		if !strings.Contains(req.Code, ":") {
			fail(w, r, http.StatusBadRequest, 40001, "errors.menu_code_scope_missing")
			return false
		}
	} else if strings.Contains(req.Code, ":") {
		fail(w, r, http.StatusBadRequest, 40001, "errors.menu_code_invalid")
		return false
	}
	if req.Status != 0 && req.Status != 1 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.menu_status_invalid")
		return false
	}
	return true
}

func isAllowedMenuType(menuType string) bool {
	switch menuType {
	case "directory", "page", "button", "external":
		return true
	default:
		return false
	}
}

func menuInput(req saveMenuRequest) store.SaveMenuInput {
	return store.SaveMenuInput{
		ParentID: req.ParentID, Type: req.Type, Name: strings.TrimSpace(req.Name), Code: strings.TrimSpace(req.Code),
		RouteName: strings.TrimSpace(req.RouteName), RoutePath: strings.TrimSpace(req.RoutePath),
		Component: strings.TrimSpace(req.Component), Redirect: strings.TrimSpace(req.Redirect),
		Icon:   strings.TrimSpace(req.Icon),
		Method: strings.ToUpper(strings.TrimSpace(req.Method)), Path: strings.TrimSpace(req.Path),
		IsSensitive: req.IsSensitive, Sort: req.Sort,
		Status: req.Status, Visible: req.Visible, KeepAlive: req.KeepAlive, AffixTab: req.AffixTab,
		ExternalURL: strings.TrimSpace(req.ExternalURL),
	}
}

func writeRBACError(w http.ResponseWriter, r *http.Request, err error, subject string) {
	failHTTP(w, r, apierrors.MapRBAC(err, subject))
}
