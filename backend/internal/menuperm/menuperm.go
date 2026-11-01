// Package menuperm turns declared route permissions into the button nodes of
// the menu tree (sys_menu rows with type='button').
//
// It is stdlib-only on purpose. store imports it to persist the buttons, and
// store is reachable from app, which plugin imports; anything heavier here
// would close the loop store -> menuperm -> ... -> app -> store.
package menuperm

import (
	"fmt"
	"strings"
)

// Route is the neutral projection of one declared route. It exists so this
// package does not import internal/plugin or internal/kernel.
type Route struct {
	// PluginID owns the button. It decides plugin gating and the sidebar
	// pruning rules the menu queries already implement.
	PluginID string
	// Codes are every permission code the route requires (Permission plus
	// AnyOf). Duplicates across routes are expected and harmless.
	Codes []string
	// MenuCode is the page the buttons hang under. Empty = derive it.
	MenuCode string
	// MenuName overrides the generated label. Empty = derive it.
	MenuName string
	Method   string
	Path     string
	// Builtin decides how an unresolvable parent is handled: a core route
	// without a page is a bug, a plugin route may simply be awaiting its
	// migration.
	Builtin bool
}

// Button is one sys_menu row to upsert.
type Button struct {
	Code      string
	Name      string
	// NameEn is the English label seeded into sys_translation at sync time.
	// It is a starting value only: the seed insert is absent-on-conflict, so
	// administrator-edited translations are never clobbered.
	NameEn    string
	Method    string
	Path      string
	PluginID  string
	ParentID  uint64
	ParentKey string
	Sensitive bool
	Sort      int
}

// Unresolved reports a permission code whose owning page could not be found.
type Unresolved struct {
	Code     string
	PluginID string
	Builtin  bool
}

// Build maps every declared permission code to one button node.
//
// menuIDs maps sys_menu.code to id for non-button, non-deleted menus. A code
// that appears on several routes yields a single button: method and path come
// from the first declaration, which keeps the result deterministic
// (declaration order is stable, and plugins are already topologically sorted).
func Build(routes []Route, menuIDs map[string]uint64) (buttons []Button, unresolved []Unresolved) {
	seen := make(map[string]bool)
	perParent := make(map[uint64]int)
	for _, route := range routes {
		for _, code := range route.Codes {
			if code == "" || seen[code] {
				continue
			}
			seen[code] = true
			parentKey := parentOf(route, code, menuIDs)
			parentID, ok := menuIDs[parentKey]
			if !ok {
				unresolved = append(unresolved, Unresolved{
					Code:     code,
					PluginID: route.PluginID,
					Builtin:  route.Builtin,
				})
				continue
			}
			perParent[parentID]++
			buttons = append(buttons, Button{
				Code:      code,
				Name:      Name(route, code),
				NameEn:    NameEn(route, code),
				Method:    route.Method,
				Path:      route.Path,
				PluginID:  route.PluginID,
				ParentID:  parentID,
				ParentKey: parentKey,
				Sensitive: route.Method != "GET",
				Sort:      perParent[parentID] * 10,
			})
		}
	}
	return buttons, unresolved
}

// parentOf prefers the explicit MenuCode and falls back to the longest
// candidate that actually exists. Deriving (instead of annotating all ~95 core
// routes) works because permission codes mirror the page they belong to:
// system:user:create -> system_user, system:dict:item:create -> system_dict.
func parentOf(route Route, code string, menuIDs map[string]uint64) string {
	if route.MenuCode != "" {
		return route.MenuCode
	}
	for _, candidate := range ParentCandidates(code) {
		if _, ok := menuIDs[candidate]; ok {
			return candidate
		}
	}
	// Nothing matched: return the longest candidate so the caller's error
	// message names something a human can act on.
	candidates := ParentCandidates(code)
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

// ParentCandidates lists the plausible page codes for a permission code,
// longest first. The trailing segment is treated as the action and dropped,
// and hyphens become underscores because that is how menu codes are spelled:
//
//	system:dict:item:create -> [system_dict_item system_dict system]
//	system:login-log:query  -> [system_login_log system]
//
// The caller takes the first one that exists, which is what makes
// system:dict:item:create land on system_dict without a special case.
func ParentCandidates(code string) []string {
	segments := strings.Split(code, ":")
	if len(segments) < 2 {
		return nil
	}
	candidates := make([]string, 0, len(segments)-1)
	for end := len(segments) - 1; end >= 1; end-- {
		parts := make([]string, 0, end)
		for _, segment := range segments[:end] {
			// Menu codes use underscores, permission codes use hyphens for
			// the same concept: system:login-log:query -> system_login_log.
			parts = append(parts, strings.ReplaceAll(segment, "-", "_"))
		}
		candidates = append(candidates, strings.Join(parts, "_"))
	}
	return candidates
}

// Name builds a readable label such as 新增用户 or 令牌管理. The route's
// MenuName always wins; when the segments are not understood the raw code is
// returned so the UI never shows an empty row.
func Name(route Route, code string) string {
	return nameFrom(route, code, actionTemplates, nounLabels)
}

// NameEn is the English twin of Name. For routes with a MenuName override the
// override is returned as-is — overriding plugins seed their own translation
// row if they ship one.
func NameEn(route Route, code string) string {
	return nameFrom(route, code, actionTemplatesEn, nounLabelsEn)
}

// actionTemplates maps the trailing code segment to a label template.
// These labels seed sys_menu.name (admin-editable data) at startup, so they
// stay in the seed language rather than going through i18n.
var actionTemplates = map[string]string{
	"query":          "查询%s",
	"create":         "新增%s",
	"update":         "修改%s",
	"delete":         "删除%s",
	"manage":         "%s管理",
	"grant":          "%s授权",
	"upload":         "上传%s",
	"reset-password": "重置%s密码",
	"force-logout":   "%s强制下线",
}

// nounLabels maps the object part of a permission code to Chinese. Multi
// segment entries (dict:item, config:storage) win over their prefix because
// the lookup uses the full object string.
var nounLabels = map[string]string{
	"user":                 "用户",
	"department":           "部门",
	"position":             "岗位",
	"role":                 "角色",
	"menu":                 "菜单",
	"dict":                 "字典",
	"dict:item":            "字典项",
	"config":               "参数",
	"config:auth-provider": "第三方登录",
	"config:storage":       "存储配置",
	"notice":               "公告",
	"file":                 "文件",
	"login-log":            "登录日志",
	"operation-log":        "操作日志",
	"error-log":            "异常日志",
	"token":                "令牌",
	"job":                  "定时任务",
	"webhook":              "网络钩子",
	"cache":                "缓存",
	"plugin":               "插件",
	"translation":          "翻译",
}

// actionTemplatesEn is the English twin of actionTemplates. Both label sets
// seed sys_translation at sync time; the stored Chinese name stays in
// sys_menu.name and English lives in the translation table, so an
// administrator can rename either language independently.
var actionTemplatesEn = map[string]string{
	"query":          "Query %s",
	"create":         "Create %s",
	"update":         "Update %s",
	"delete":         "Delete %s",
	"manage":         "%s Management",
	"grant":          "Grant %s",
	"upload":         "Upload %s",
	"reset-password": "Reset %s password",
	"force-logout":   "Force %s offline",
}

// nounLabelsEn is the English twin of nounLabels.
var nounLabelsEn = map[string]string{
	"user":                 "User",
	"department":           "Department",
	"position":             "Position",
	"role":                 "Role",
	"menu":                 "Menu",
	"dict":                 "Dictionary",
	"dict:item":            "Dictionary Item",
	"config":               "Config",
	"config:auth-provider": "Third-party Login",
	"config:storage":       "Storage Config",
	"notice":               "Notice",
	"file":                 "File",
	"login-log":            "Login Log",
	"operation-log":        "Operation Log",
	"error-log":            "Error Log",
	"token":                "Token",
	"job":                  "Scheduled Job",
	"webhook":              "Webhook",
	"cache":                "Cache",
	"plugin":               "Plugin",
	"translation":          "Translation",
}

// nameFrom builds a label with the given template/noun tables. Shared by
// Name (Chinese, stored in sys_menu.name) and NameEn (English, stored in
// sys_translation); see Name for the derivation rules.
func nameFrom(route Route, code string, actionTemplates, nounLabels map[string]string) string {
	if route.MenuName != "" {
		return route.MenuName
	}
	segments := strings.Split(code, ":")
	if len(segments) < 2 {
		return code
	}
	noun := segments[1:]
	verb := ""
	if template, ok := actionTemplates[segments[len(segments)-1]]; ok {
		noun = segments[1 : len(segments)-1]
		verb = template
	}
	label := nounLabels[strings.Join(noun, ":")]
	if label == "" {
		return code
	}
	if verb == "" {
		return label
	}
	return fmt.Sprintf(verb, label)
}
