package kernel

import (
	"dragz-admin/backend/internal/plugin"
)

// AuditAttribution resolves the permission code recorded in audit rows:
// explicit AuditPermission first, then Permission, then AnyOf[0]. Routes
// without any code (login-only mutations) record an empty permission.
func AuditAttribution(route plugin.Route) string {
	return auditAttribution(route)
}

func auditAttribution(route plugin.Route) string {
	if route.AuditPermission != "" {
		return route.AuditPermission
	}
	if route.Permission != "" {
		return route.Permission
	}
	if len(route.AnyOf) > 0 {
		return route.AnyOf[0]
	}
	return ""
}
