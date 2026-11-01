// Package casbinx wraps casbin with the platform's fixed 3-tuple policy model
// (role, permission code, access).
package casbinx

import (
	"fmt"
	"strconv"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
)

const defaultModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.obj == p.obj && r.act == p.act
`

// Policy is one role-to-code rule loaded into the enforcer.
type Policy struct {
	RoleID         uint64
	PermissionCode string
}

type memoryAdapter struct{}

var _ persist.BatchAdapter = memoryAdapter{}

func (memoryAdapter) LoadPolicy(_ model.Model) error { return nil }
func (memoryAdapter) SavePolicy(_ model.Model) error { return nil }
func (memoryAdapter) AddPolicy(string, string, []string) error {
	return nil
}
func (memoryAdapter) RemovePolicy(string, string, []string) error {
	return nil
}
func (memoryAdapter) RemoveFilteredPolicy(string, string, int, ...string) error {
	return nil
}
func (memoryAdapter) AddPolicies(string, string, [][]string) error {
	return nil
}
func (memoryAdapter) RemovePolicies(string, string, [][]string) error {
	return nil
}

// Enforcer wraps casbin with the fixed 3-tuple model (role, permission code, access) used across the platform. Policies live in memory only: they are rebuilt from the database on startup and after every grant change.
type Enforcer struct {
	inner *casbin.Enforcer
}

// NewEnforcer builds an empty enforcer backed by a no-op adapter, so casbin never tries to persist anything itself.
func NewEnforcer() (*Enforcer, error) {
	m, err := model.NewModelFromString(defaultModel)
	if err != nil {
		return nil, err
	}
	inner, err := casbin.NewEnforcer(m, memoryAdapter{})
	if err != nil {
		return nil, err
	}
	inner.EnableAutoSave(false)
	return &Enforcer{inner: inner}, nil
}

// ReplacePolicies swaps the whole policy set atomically from the caller's point of view. An empty slice clears the enforcer, which is what a role with no grants should do.
func (e *Enforcer) ReplacePolicies(policies []Policy) error {
	if e == nil || e.inner == nil {
		return fmt.Errorf("casbin enforcer is not initialized")
	}
	e.inner.ClearPolicy()
	rules := make([][]string, 0, len(policies))
	for _, policy := range policies {
		rules = append(rules, []string{
			strconv.FormatUint(policy.RoleID, 10),
			policy.PermissionCode,
			"access",
		})
	}
	if len(rules) == 0 {
		return nil
	}
	_, err := e.inner.AddPolicies(rules)
	return err
}

// AllowAny reports whether any of the roles grants the code. A nil enforcer, an empty code or an empty role list denies by default.
func (e *Enforcer) AllowAny(roleIDs []uint64, permissionCode string) bool {
	if e == nil || e.inner == nil || permissionCode == "" {
		return false
	}
	for _, roleID := range roleIDs {
		allowed, err := e.inner.Enforce(strconv.FormatUint(roleID, 10), permissionCode, "access")
		if err == nil && allowed {
			return true
		}
	}
	return false
}
