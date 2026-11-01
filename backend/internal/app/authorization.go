package app

import (
	"context"
	"encoding/json"
	"fmt"

	"dragz-admin/backend/internal/pkg/casbinx"
)

const permissionCachePrefix = "cache:perm:user:"

// ReloadAuthorization rebuilds the Casbin policy set from the database. It must run after the plugin startup DDL, because the policy query joins sys_plugin.
func (c *Context) ReloadAuthorization(ctx context.Context) error {
	policies, err := c.Auth.ListCasbinPolicies(ctx)
	if err != nil {
		return err
	}
	if c.Casbin == nil {
		return fmt.Errorf("casbin enforcer is not initialized")
	}
	casbinPolicies := make([]casbinx.Policy, 0, len(policies))
	for _, policy := range policies {
		casbinPolicies = append(casbinPolicies, casbinx.Policy{
			RoleID:         policy.RoleID,
			PermissionCode: policy.PermissionCode,
		})
	}
	return c.Casbin.ReplacePolicies(casbinPolicies)
}

// UserRoleIDs returns the enabled roles of a user, which is the subject set the enforcer is asked about.
func (c *Context) UserRoleIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	return c.Auth.UserRoleIDs(ctx, userID)
}

// PermissionAllowed decides whether a user holds a permission code. A super administrator short-circuits to true; otherwise the user's role list is read from a short-lived Redis cache and evaluated against the in-memory enforcer.
func (c *Context) PermissionAllowed(ctx context.Context, userID uint64, superAdmin bool, permission string) (bool, error) {
	if superAdmin {
		return true, nil
	}
	if cached, ok, err := c.cachedPermissionAllowed(ctx, userID, permission); err != nil {
		return false, err
	} else if ok {
		return cached, nil
	}
	roleIDs, err := c.Auth.UserRoleIDs(ctx, userID)
	if err != nil {
		return false, err
	}
	allowed := c.Casbin != nil && c.Casbin.AllowAny(roleIDs, permission)
	if err := c.cacheUserPermissions(ctx, userID, roleIDs); err != nil {
		return allowed, err
	}
	return allowed, nil
}

func (c *Context) cachedPermissionAllowed(ctx context.Context, userID uint64, permission string) (bool, bool, error) {
	if c.Cache == nil {
		return false, false, nil
	}
	value, err := c.Cache.Get(permissionCachePrefix + fmt.Sprint(userID))
	if err != nil || value == "" {
		return false, false, nil
	}
	var roleIDs []uint64
	if err := json.Unmarshal([]byte(value), &roleIDs); err != nil {
		return false, false, nil
	}
	return c.Casbin.AllowAny(roleIDs, permission), true, nil
}

func (c *Context) cacheUserPermissions(ctx context.Context, userID uint64, roleIDs []uint64) error {
	if c.Cache == nil {
		return nil
	}
	payload, err := json.Marshal(roleIDs)
	if err != nil {
		return err
	}
	return c.Cache.Setex(permissionCachePrefix+fmt.Sprint(userID), string(payload), 300)
}

// InvalidateUserPermissionCache drops the cached role list of one user, which is how a grant change takes effect immediately.
func (c *Context) InvalidateUserPermissionCache(userID uint64) {
	if c.Cache == nil {
		return
	}
	_, _ = c.Cache.Del(permissionCachePrefix + fmt.Sprint(userID))
}

// AfterRoleGrantsChanged reloads the policies and drops the cache of the given users. It is the single entry point every grant mutation calls.
func (c *Context) AfterRoleGrantsChanged(ctx context.Context, userIDs ...uint64) error {
	if err := c.ReloadAuthorization(ctx); err != nil {
		return err
	}
	for _, userID := range userIDs {
		c.InvalidateUserPermissionCache(userID)
	}
	return nil
}
