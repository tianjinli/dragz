package store

import (
	"context"
	"sort"
)

// DataScope is the effective union of all enabled roles assigned to a user.
// All bypasses the remaining fields. SelfUserID and DepartmentIDs are OR-ed.
type DataScope struct {
	All           bool
	SelfUserID    uint64
	DepartmentIDs []uint64
}

// AllowsDepartment reports whether the scope grants access to a department.
func (scope DataScope) AllowsDepartment(departmentID uint64) bool {
	if scope.All {
		return true
	}
	for _, allowedID := range scope.DepartmentIDs {
		if allowedID == departmentID {
			return true
		}
	}
	return false
}

// ResolveUserDataScope unions the data scopes of every enabled role the user holds. A super administrator, or any role with scope "all", short-circuits to unrestricted access; a user with no role at all falls back to "self".
func (r *AuthRepository) ResolveUserDataScope(ctx context.Context, userID uint64) (DataScope, error) {
	basics, err := r.queries.GetUserScopeBasics(ctx, int64(userID))
	if err != nil {
		return DataScope{}, err
	}
	if smallIntBool(basics.IsSuperAdmin) {
		return DataScope{All: true}, nil
	}

	rows, err := r.queries.ListUserRoleDataScopes(ctx, userID)
	if err != nil {
		return DataScope{}, err
	}

	scope := DataScope{}
	departments := make(map[uint64]struct{})
	hasRule := false
	for _, row := range rows {
		hasRule = true
		switch row.DataScope {
		case "all":
			return DataScope{All: true}, nil
		case "self":
			scope.SelfUserID = userID
		case "department":
			if basics.DepartmentID.Valid {
				departments[uint64(basics.DepartmentID.Int64)] = struct{}{}
			}
		case "department_and_children":
			if basics.DepartmentID.Valid {
				if err := r.addDepartmentAndChildren(ctx, uint64(basics.DepartmentID.Int64), departments); err != nil {
					return DataScope{}, err
				}
			}
		case "custom":
			if err := r.addCustomRoleDepartments(ctx, uint64(row.ID), departments); err != nil {
				return DataScope{}, err
			}
		}
	}
	if !hasRule {
		scope.SelfUserID = userID
	}
	for id := range departments {
		scope.DepartmentIDs = append(scope.DepartmentIDs, id)
	}
	sort.Slice(scope.DepartmentIDs, func(i, j int) bool {
		return scope.DepartmentIDs[i] < scope.DepartmentIDs[j]
	})
	return scope, nil
}

func (r *AuthRepository) addDepartmentAndChildren(ctx context.Context, departmentID uint64, target map[uint64]struct{}) error {
	ids, err := r.queries.ListDepartmentAndChildrenIDs(ctx, int64(departmentID))
	if err != nil {
		return err
	}
	for _, id := range ids {
		target[uint64(id)] = struct{}{}
	}
	return nil
}

func (r *AuthRepository) addCustomRoleDepartments(ctx context.Context, roleID uint64, target map[uint64]struct{}) error {
	ids, err := r.queries.ListCustomRoleDepartmentIDs(ctx, roleID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		target[id] = struct{}{}
	}
	return nil
}

// DataScopeAllowsUser reports whether the scope covers another user, either directly (self) or through that user's department.
func (r *AuthRepository) DataScopeAllowsUser(ctx context.Context, scope DataScope, targetUserID uint64) (bool, error) {
	if scope.All || scope.SelfUserID == targetUserID {
		return true, nil
	}
	departmentID, err := r.queries.GetUserDepartmentID(ctx, int64(targetUserID))
	if err != nil {
		return false, err
	}
	return departmentID.Valid && scope.AllowsDepartment(uint64(departmentID.Int64)), nil
}
