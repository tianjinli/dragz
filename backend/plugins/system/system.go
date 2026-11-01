// Package system is the first built-in plugin: user/role/menu/permission,
// dictionary, log, job, file and other platform capabilities.
//
// P0: the handlers still live in internal/httpapi; this package re-exports
// their declarations. The import of internal/httpapi is the ONLY dependency
// exception (check-imports allowlist) and disappears in P6 when the handlers
// move here.
package system

import (
	"dragz-admin/backend/internal/httpapi"
	"dragz-admin/backend/internal/plugin"
)

// Plugin implements plugin.Plugin for the system module.
type Plugin struct{}

func (Plugin) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "system",
		Name:        "系统管理",
		Version:     "1.0.0",
		Description: "用户、角色、菜单、权限、字典、配置、日志、任务、文件、缓存等平台基础能力",
		Builtin:     true,
	}
}

func (Plugin) Routes() []plugin.Route { return httpapi.Routes() }
