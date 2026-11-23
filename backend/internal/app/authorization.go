package app

import (
	"context"
	"encoding/json"
	"fmt"

	"dragz-admin/backend/internal/pkg/casbinx"
	"dragz-admin/backend/internal/pkg/ctxkey"
)

const permissionCachePrefix = "cache:perm:user:"

// ReloadAuthorization rebuilds the Casbin policy set from the database. It must run after the plugin startup DDL, because the policy query joins sys_plugin.
// The reload always bypasses row-level security and ignores any request transaction, so one tenant's session cannot replace the enforcer with a partial policy set.
func (c *Context) ReloadAuthorization(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = ctxkey.WithTx(ctx, nil)
	ctx = ctxkey.WithRLSBypass(ctx, true)
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
			TenantID:       policy.TenantID,
			PermissionCode: policy.PermissionCode,
		})
	}
	return c.Casbin.ReplacePolicies(casbinPolicies)
}

// UserRoleIDs returns the enabled roles of a user inside the tenant on ctx,
// which is the subject set the enforcer is asked about.
func (c *Context) UserRoleIDs(ctx context.Context, userID uint64) ([]uint64, error) {
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	return c.Auth.UserRoleIDs(ctx, userID, tenantID)
}

// PermissionAllowed decides whether a user holds a permission code. A super administrator short-circuits to true; otherwise the user's role list is read from a short-lived Redis cache and evaluated against the in-memory enforcer.
func (c *Context) PermissionAllowed(ctx context.Context, userID uint64, superAdmin bool, permission string) (bool, error) {
	if superAdmin {
		return true, nil
	}
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	if cached, ok, err := c.cachedPermissionAllowed(ctx, userID, tenantID, permission); err != nil {
		return false, err
	} else if ok {
		return cached, nil
	}
	roleIDs, err := c.Auth.UserRoleIDs(ctx, userID, tenantID)
	if err != nil {
		return false, err
	}
	allowed := c.Casbin != nil && c.Casbin.AllowAny(roleIDs, tenantID, permission)
	if err := c.cacheUserPermissions(ctx, userID, tenantID, roleIDs); err != nil {
		return allowed, err
	}
	return allowed, nil
}

// PermissionCacheKeyPattern is the scan pattern that matches every permission
// cache entry of one tenant. Tenant comes before user in the key so the pattern
// cannot reach into another tenant's namespace.
func PermissionCacheKeyPattern(tenantID uint64) string {
	return fmt.Sprintf("%s%d:*", permissionCachePrefix, tenantID)
}

// PermissionCacheKeyPrefix is the literal namespace of one tenant's cache keys.
func PermissionCacheKeyPrefix(tenantID uint64) string {
	return fmt.Sprintf("%s%d:", permissionCachePrefix, tenantID)
}

// permissionCacheKey namespaces by tenant first. The cache administration
// endpoints scan by prefix, so a user-first key would let one tenant list and
// delete another tenant's entries.
func permissionCacheKey(userID, tenantID uint64) string {
	return fmt.Sprintf("%s%d:%d", permissionCachePrefix, tenantID, userID)
}

func (c *Context) cachedPermissionAllowed(ctx context.Context, userID, tenantID uint64, permission string) (bool, bool, error) {
	if c.Cache == nil {
		return false, false, nil
	}
	value, err := c.Cache.Get(permissionCacheKey(userID, tenantID))
	if err != nil || value == "" {
		return false, false, nil
	}
	var roleIDs []uint64
	if err := json.Unmarshal([]byte(value), &roleIDs); err != nil {
		return false, false, nil
	}
	return c.Casbin.AllowAny(roleIDs, tenantID, permission), true, nil
}

func (c *Context) cacheUserPermissions(ctx context.Context, userID, tenantID uint64, roleIDs []uint64) error {
	if c.Cache == nil {
		return nil
	}
	payload, err := json.Marshal(roleIDs)
	if err != nil {
		return err
	}
	return c.Cache.Setex(permissionCacheKey(userID, tenantID), string(payload), 300)
}

// InvalidateUserPermissionCache drops the cached role list of one user, which is how a grant change takes effect immediately.
func (c *Context) InvalidateUserPermissionCache(userID uint64) {
	if c.Cache == nil {
		return
	}
	if keys, err := c.Cache.Keys(permissionCachePrefix + "*:" + fmt.Sprint(userID)); err == nil && len(keys) > 0 {
		_, _ = c.Cache.Del(keys...)
	}
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
