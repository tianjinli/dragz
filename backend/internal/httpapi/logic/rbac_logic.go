package logic

import (
	"context"

	"dragz-admin/backend/internal/httpapi/svc"
	"dragz-admin/backend/internal/store"
)

// RBACLogic is the orchestration layer for roles and menus. It owns the authorization reload that has to follow a grant change; the handlers stay thin.
type RBACLogic struct {
	svcCtx *svc.ServiceContext
}

// NewRBACLogic binds the logic layer to the service context.
func NewRBACLogic(svcCtx *svc.ServiceContext) *RBACLogic {
	return &RBACLogic{svcCtx: svcCtx}
}

// ListRoles returns every role for the role management page.
func (l *RBACLogic) ListRoles(ctx context.Context, keyword string, status *int8) ([]store.Role, error) {
	return l.svcCtx.Auth.ListRoles(ctx, keyword, status)
}

// CreateRole adds a role.
func (l *RBACLogic) CreateRole(ctx context.Context, input store.SaveRoleInput) (uint64, error) {
	return l.svcCtx.Auth.CreateRole(ctx, input)
}

// UpdateRole rewrites a role; the built-in super administrator is refused.
func (l *RBACLogic) UpdateRole(ctx context.Context, roleID uint64, input store.SaveRoleInput) error {
	return l.svcCtx.Auth.UpdateRole(ctx, roleID, input)
}

// UpdateRoleStatus enables or disables a role.
func (l *RBACLogic) UpdateRoleStatus(ctx context.Context, roleID uint64, status int8) error {
	return l.svcCtx.Auth.UpdateRoleStatus(ctx, roleID, status)
}

// DeleteRole removes a role that is neither a system role nor still assigned.
func (l *RBACLogic) DeleteRole(ctx context.Context, roleID uint64) error {
	return l.svcCtx.Auth.DeleteRole(ctx, roleID)
}

// GetRoleGrants returns the single grant tree of a role: menu nodes and permission buttons together.
func (l *RBACLogic) GetRoleGrants(ctx context.Context, roleID uint64) (store.RoleGrants, error) {
	return l.svcCtx.Auth.GetRoleGrants(ctx, roleID)
}

// SaveRoleGrants replaces the whole grant set and reloads the enforcer, so the new grants apply to the next request.
func (l *RBACLogic) SaveRoleGrants(ctx context.Context, grants store.RoleGrants) error {
	return l.svcCtx.Auth.SaveRoleGrants(ctx, grants)
}

// AfterRoleGrantsChanged reloads the Casbin policies from the database and drops the permission cache.
func (l *RBACLogic) AfterRoleGrantsChanged(ctx context.Context) error {
	return l.svcCtx.AfterRoleGrantsChanged(ctx)
}

// ListMenuResources returns the flat menu table the admin UI renders as a tree.
func (l *RBACLogic) ListMenuResources(ctx context.Context) ([]store.MenuResource, error) {
	return l.svcCtx.Auth.ListMenuResources(ctx)
}

// CreateMenu adds a menu node, including a hand-made permission button.
func (l *RBACLogic) CreateMenu(ctx context.Context, input store.SaveMenuInput) (uint64, error) {
	return l.svcCtx.Auth.CreateMenu(ctx, input)
}

// UpdateMenu rewrites a menu node.
func (l *RBACLogic) UpdateMenu(ctx context.Context, menuID uint64, input store.SaveMenuInput) error {
	return l.svcCtx.Auth.UpdateMenu(ctx, menuID, input)
}

// UpdateMenuStatus enables or disables a menu node.
func (l *RBACLogic) UpdateMenuStatus(ctx context.Context, menuID uint64, status int8) error {
	return l.svcCtx.Auth.UpdateMenuStatus(ctx, menuID, status)
}

// DeleteMenu removes a menu node and its grants.
func (l *RBACLogic) DeleteMenu(ctx context.Context, menuID uint64) error {
	return l.svcCtx.Auth.DeleteMenu(ctx, menuID)
}
