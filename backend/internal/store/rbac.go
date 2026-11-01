package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// Role is one sys_role row together with its user count.
type Role struct {
	ID        uint64    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	DataScope string    `json:"dataScope"`
	Sort      int       `json:"sort"`
	Status    int8      `json:"status"`
	IsSystem  bool      `json:"isSystem"`
	Remark    string    `json:"remark"`
	UserCount int64     `json:"userCount"`
	CreatedAt time.Time `json:"createdAt"`
}

// SaveRoleInput is the payload shared by create and update.
type SaveRoleInput struct {
	Code      string
	Name      string
	DataScope string
	Sort      int
	Status    int8
	Remark    string
}

// RoleGrants is the single grant tree: menu nodes and permission buttons are
// both rows of sys_menu, so there is no separate permission id list.
type RoleGrants struct {
	RoleID        uint64   `json:"roleId"`
	DataScope     string   `json:"dataScope"`
	MenuIDs       []uint64 `json:"menuIds"`
	DepartmentIDs []uint64 `json:"departmentIds"`
}

// MenuResource is one sys_menu row as the menu manager shows it, buttons included.
type MenuResource struct {
	ID          uint64 `json:"id"`
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
	// Managed marks a row the route-table sync owns (a declared permission
	// code). The admin UI uses it to lock the fields the sync overwrites.
	Managed     bool      `json:"managed"`
	Sort        int       `json:"sort"`
	Status      int8      `json:"status"`
	Visible     bool      `json:"visible"`
	KeepAlive   bool      `json:"keepAlive"`
	AffixTab    bool      `json:"affixTab"`
	ExternalURL string    `json:"externalUrl"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SaveMenuInput is the payload shared by create and update.
type SaveMenuInput struct {
	ParentID    uint64
	Type        string
	Name        string
	Code        string
	RouteName   string
	RoutePath   string
	Component   string
	Redirect    string
	Icon        string
	Method      string
	Path        string
	IsSensitive bool
	Sort        int
	Status      int8
	Visible     bool
	KeepAlive   bool
	AffixTab    bool
	ExternalURL string
}

// ListRoles returns every role filtered by keyword and status. A nil status means "any status".
func (r *AuthRepository) ListRoles(ctx context.Context, keyword string, status *int8) ([]Role, error) {
	rows, err := r.queries.ListRolesFiltered(ctx, sqlcstore.ListRolesFilteredParams{
		KeywordPattern: optionalPattern(keyword), Status: optionalInt16(status),
	})
	if err != nil {
		return nil, err
	}
	items := make([]Role, 0, len(rows))
	for _, row := range rows {
		items = append(items, Role{
			ID: uint64(row.ID), Code: row.Code, Name: row.Name, DataScope: row.DataScope,
			Sort: int(row.Sort), Status: int8(row.Status), IsSystem: smallIntBool(row.IsSystem),
			Remark: row.Remark, UserCount: row.UserCount, CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

// CreateRole inserts a role and reports a code clash as ErrDuplicateEntry.
func (r *AuthRepository) CreateRole(ctx context.Context, input SaveRoleInput) (uint64, error) {
	id, err := r.queries.CreateRole(ctx, sqlcstore.CreateRoleParams{
		Code: input.Code, Name: input.Name, DataScope: input.DataScope,
		Sort: int32(input.Sort), Status: int16(input.Status), Remark: input.Remark,
	})
	return uint64(id), wrapDuplicate(err)
}

// UpdateRole rewrites a role. The built-in super administrator is refused by the query's WHERE clause and surfaces as ErrRoleProtected.
func (r *AuthRepository) UpdateRole(ctx context.Context, roleID uint64, input SaveRoleInput) error {
	affected, err := r.queries.UpdateRole(ctx, sqlcstore.UpdateRoleParams{
		Code: input.Code, Name: input.Name, DataScope: input.DataScope,
		Sort: int32(input.Sort), Status: int16(input.Status), Remark: input.Remark, ID: int64(roleID),
	})
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrRoleProtected
	}
	return nil
}

// UpdateRoleStatus enables or disables a role.
func (r *AuthRepository) UpdateRoleStatus(ctx context.Context, roleID uint64, status int8) error {
	affected, err := r.queries.UpdateRoleStatus(ctx, sqlcstore.UpdateRoleStatusParams{
		Status: int16(status), ID: int64(roleID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrRoleProtected
	}
	return nil
}

// DeleteRole soft deletes a role. System roles and roles still assigned to a user are refused with ErrProtected.
func (r *AuthRepository) DeleteRole(ctx context.Context, roleID uint64) error {
	checks, err := r.queries.GetRoleDeleteChecks(ctx, int64(roleID))
	if err != nil {
		return err
	}
	if smallIntBool(checks.IsSystem) || checks.UserCount > 0 {
		return ErrProtected
	}
	affected, err := r.queries.SoftDeleteRole(ctx, int64(roleID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// GetRoleGrants loads the single grant tree of a role: menu ids (pages and buttons alike), the data scope and the custom departments.
func (r *AuthRepository) GetRoleGrants(ctx context.Context, roleID uint64) (RoleGrants, error) {
	grants := RoleGrants{RoleID: roleID}
	var err error
	grants.DataScope, err = r.queries.GetRoleDataScope(ctx, int64(roleID))
	if err != nil {
		return RoleGrants{}, err
	}
	grants.MenuIDs, err = r.queries.ListRoleMenuIDs(ctx, roleID)
	if err != nil {
		return RoleGrants{}, err
	}
	grants.DepartmentIDs, err = r.queries.ListRoleDepartmentIDs(ctx, roleID)
	return grants, err
}

// SaveRoleGrants replaces the whole grant set in one transaction. The built-in super administrator is refused with ErrProtected.
func (r *AuthRepository) SaveRoleGrants(ctx context.Context, grants RoleGrants) error {
	isSystem, err := r.queries.GetRoleIsSystem(ctx, int64(grants.RoleID))
	if err != nil {
		return err
	}
	if smallIntBool(isSystem) {
		return ErrProtected
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := q.UpdateRoleDataScope(ctx, sqlcstore.UpdateRoleDataScopeParams{
		DataScope: grants.DataScope, ID: int64(grants.RoleID),
	}); err != nil {
		return err
	}
	if err := q.DeleteRoleMenus(ctx, grants.RoleID); err != nil {
		return err
	}
	for _, menuID := range distinctIDs(grants.MenuIDs) {
		if err := q.InsertRoleMenu(ctx, sqlcstore.InsertRoleMenuParams{RoleID: grants.RoleID, MenuID: menuID}); err != nil {
			return err
		}
	}
	if err := q.DeleteRoleDepartments(ctx, grants.RoleID); err != nil {
		return err
	}
	for _, departmentID := range distinctIDs(grants.DepartmentIDs) {
		if err := q.InsertRoleDepartment(ctx, sqlcstore.InsertRoleDepartmentParams{RoleID: grants.RoleID, DepartmentID: departmentID}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListMenuResources returns the flat menu table, which the admin UI turns into a tree.
func (r *AuthRepository) ListMenuResources(ctx context.Context) ([]MenuResource, error) {
	rows, err := r.queries.ListMenuResources(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]MenuResource, 0, len(rows))
	for _, row := range rows {
		items = append(items, MenuResource{
			ID: uint64(row.ID), ParentID: row.ParentID, Type: row.Type, Name: row.Name, Code: row.Code,
			RouteName: row.RouteName, RoutePath: row.RoutePath, Component: row.Component, Redirect: row.Redirect,
			Icon: row.Icon, Sort: int(row.Sort), Status: int8(row.Status),
			Method: row.Method, Path: row.Path, IsSensitive: smallIntBool(row.IsSensitive),
			Managed: smallIntBool(row.Managed),
			Visible: smallIntBool(row.Visible), KeepAlive: smallIntBool(row.KeepAlive),
			AffixTab: smallIntBool(row.AffixTab), ExternalURL: row.ExternalUrl, CreatedAt: row.CreatedAt,
		})
	}
	return items, nil
}

// lockManagedMenuFields overwrites every field the route-table sync owns when
// the target row is a sync-owned permission button. Only name and sort survive,
// which is what the admin UI is allowed to change.
//
// A button created by hand (managed = 0) is a normal row: the administrator may
// edit it, disable it and delete it. The only thing pinned is the type, so a
// button can never turn into a page that shadows a route path.
func (r *AuthRepository) lockManagedMenuFields(ctx context.Context, menuID uint64, input SaveMenuInput) (SaveMenuInput, error) {
	menuType, err := r.queries.GetMenuType(ctx, int64(menuID))
	if errors.Is(err, sql.ErrNoRows) {
		return input, ErrNotFound
	}
	if err != nil {
		return input, err
	}
	if menuType != "button" {
		return input, nil
	}
	owned, err := r.queries.GetMenuManagedFields(ctx, int64(menuID))
	if err != nil {
		return input, err
	}
	if !smallIntBool(owned.Managed) {
		input.Type = owned.Type
		return input, nil
	}
	return SaveMenuInput{
		ParentID:    uint64(owned.ParentID),
		Type:        owned.Type,
		Name:        input.Name,
		Code:        owned.Code,
		Method:      owned.Method,
		Path:        owned.Path,
		IsSensitive: smallIntBool(owned.IsSensitive),
		Sort:        input.Sort,
		Status:      int8(owned.Status),
	}, nil
}

// validateButtonParent keeps a permission button under a real directory or
// page. A button may never own another button, and a root-level or dangling
// parent is refused too — the admin UI only offers valid parents, so this
// catches API callers.
func (r *AuthRepository) validateButtonParent(ctx context.Context, input SaveMenuInput) error {
	if input.Type != "button" {
		return nil
	}
	if input.ParentID == 0 {
		return ErrMenuParentInvalid
	}
	parentType, err := r.queries.GetMenuType(ctx, int64(input.ParentID))
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMenuParentInvalid
	}
	if err != nil {
		return err
	}
	if parentType == "button" {
		return ErrMenuParentInvalid
	}
	return nil
}

// CreateMenu inserts a menu row. A hand-made row always gets managed = 0, so the route-table sync never claims it.
func (r *AuthRepository) CreateMenu(ctx context.Context, input SaveMenuInput) (uint64, error) {
	if err := r.validateButtonParent(ctx, input); err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	menuID, err := q.CreateMenu(ctx, sqlcstore.CreateMenuParams{
		ParentID: input.ParentID, Type: input.Type, Name: input.Name, Code: input.Code,
		RouteName: input.RouteName, RoutePath: input.RoutePath, Component: input.Component,
		Redirect: input.Redirect, Icon: input.Icon,
		Method: input.Method, Path: input.Path, IsSensitive: boolSmallInt16(input.IsSensitive),
		Sort: int32(input.Sort), Status: int16(input.Status), Visible: boolSmallInt16(input.Visible),
		KeepAlive: boolSmallInt16(input.KeepAlive), AffixTab: boolSmallInt16(input.AffixTab),
		ExternalUrl: input.ExternalURL,
	})
	if err != nil {
		return 0, wrapDuplicate(err)
	}
	return uint64(menuID), tx.Commit()
}

// UpdateMenu rewrites a menu row after pinning the fields the sync owns and rejecting a move below itself.
func (r *AuthRepository) UpdateMenu(ctx context.Context, menuID uint64, input SaveMenuInput) error {
	input, err := r.lockManagedMenuFields(ctx, menuID, input)
	if err != nil {
		return err
	}
	if err := r.validateButtonParent(ctx, input); err != nil {
		return err
	}
	if menuID == input.ParentID {
		return ErrMenuCycle
	}
	isDescendant, err := r.queries.MenuParentIsDescendant(ctx, sqlcstore.MenuParentIsDescendantParams{
		MenuID: menuID, ParentID: input.ParentID,
	})
	if err != nil {
		return err
	}
	if isDescendant {
		return ErrMenuDescendant
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.UpdateMenu(ctx, sqlcstore.UpdateMenuParams{
		ParentID: input.ParentID, Type: input.Type, Name: input.Name, Code: input.Code,
		RouteName: input.RouteName, RoutePath: input.RoutePath, Component: input.Component,
		Redirect: input.Redirect, Icon: input.Icon,
		Method: input.Method, Path: input.Path, IsSensitive: boolSmallInt16(input.IsSensitive),
		Sort: int32(input.Sort), Status: int16(input.Status), Visible: boolSmallInt16(input.Visible),
		KeepAlive: boolSmallInt16(input.KeepAlive), AffixTab: boolSmallInt16(input.AffixTab),
		ExternalUrl: input.ExternalURL, ID: int64(menuID),
	})
	if err != nil {
		return wrapDuplicate(err)
	}
	if affected != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

// menuOwnedBySync reports whether the row's fields belong to the route-table
// sync. Only sync-owned rows reject status changes and deletion; a button the
// administrator created is theirs to manage.
func (r *AuthRepository) menuOwnedBySync(ctx context.Context, menuID uint64) (bool, error) {
	owned, err := r.queries.GetMenuManagedFields(ctx, int64(menuID))
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return smallIntBool(owned.Managed), nil
}

// UpdateMenuStatus enables or disables a menu. A sync-owned button is refused with ErrMenuButtonManaged.
func (r *AuthRepository) UpdateMenuStatus(ctx context.Context, menuID uint64, status int8) error {
	owned, err := r.menuOwnedBySync(ctx, menuID)
	if err != nil {
		return err
	}
	if owned {
		// A sync-owned button's status follows its route: it exists because a
		// route exists, so "disabling" it would be undone on the next boot.
		return ErrMenuButtonManaged
	}
	affected, err := r.queries.UpdateMenuStatus(ctx, sqlcstore.UpdateMenuStatusParams{
		Status: int16(status), ID: int64(menuID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// DeleteMenu soft deletes a menu and drops its role grants in one transaction. A sync-owned button is refused with ErrMenuButtonManaged.
func (r *AuthRepository) DeleteMenu(ctx context.Context, menuID uint64) error {
	owned, err := r.menuOwnedBySync(ctx, menuID)
	if err != nil {
		return err
	}
	if owned {
		return ErrMenuButtonManaged
	}
	childCount, err := r.queries.CountMenuChildren(ctx, menuID)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrMenuHasChildren
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.SoftDeleteMenu(ctx, int64(menuID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	if err := q.DeleteRoleMenusByMenu(ctx, menuID); err != nil {
		return err
	}
	return tx.Commit()
}

// ListCasbinPolicies returns the (role, code) pairs the enforcer has to load, read from the button rows of granted menus.
func (r *AuthRepository) ListCasbinPolicies(ctx context.Context) ([]RolePermissionPolicy, error) {
	rows, err := r.queries.ListCasbinPolicies(ctx)
	if err != nil {
		return nil, err
	}
	policies := make([]RolePermissionPolicy, 0, len(rows))
	for _, row := range rows {
		policies = append(policies, RolePermissionPolicy{RoleID: row.RoleID, PermissionCode: row.Code})
	}
	return policies, nil
}
