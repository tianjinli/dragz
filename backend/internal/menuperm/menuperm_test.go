package menuperm_test

import (
	"testing"

	"dragz-admin/backend/internal/menuperm"
)

func TestParentCandidates(t *testing.T) {
	cases := map[string][]string{
		"system:user:create":      {"system_user", "system"},
		"system:dict:item:create": {"system_dict_item", "system_dict", "system"},
		"system:config:storage":   {"system_config", "system"},
		// Hyphens in permission codes map to underscores in menu codes.
		"system:login-log:query": {"system_login_log", "system"},
		"example:ping":           {"example"},
		"ping":                   nil,
	}
	for code, want := range cases {
		got := menuperm.ParentCandidates(code)
		if len(got) != len(want) {
			t.Fatalf("%s: got %v want %v", code, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v want %v", code, got, want)
			}
		}
	}
}

func TestName(t *testing.T) {
	cases := map[string]string{
		"system:user:create":          "新增用户",
		"system:user:query":           "查询用户",
		"system:user:reset-password":  "重置用户密码",
		"system:dict:item:create":     "新增字典项",
		"system:token:manage":         "令牌管理",
		"system:role:grant":           "角色授权",
		"system:file:upload":          "上传文件",
		"system:config:auth-provider": "第三方登录",
		"system:config:storage":       "存储配置",
		"system:cache:manage":         "缓存管理",
		"system:plugin:query":         "查询插件",
		// Unknown segments fall back to the raw code so the UI never shows
		// an empty row.
		"unknown:thing:create": "unknown:thing:create",
		"example:ping":         "example:ping",
	}
	for code, want := range cases {
		if got := menuperm.Name(menuperm.Route{}, code); got != want {
			t.Errorf("%s: got %q want %q", code, got, want)
		}
	}
}

func TestNamePrefersRouteOverride(t *testing.T) {
	route := menuperm.Route{MenuName: "连通性检查"}
	if got := menuperm.Name(route, "example:ping"); got != "连通性检查" {
		t.Fatalf("got %q", got)
	}
}

func TestNameEn(t *testing.T) {
	cases := map[string]string{
		"system:user:create":          "Create User",
		"system:user:query":           "Query User",
		"system:user:reset-password":  "Reset User password",
		"system:dict:item:create":     "Create Dictionary Item",
		"system:token:manage":         "Token Management",
		"system:role:grant":           "Grant Role",
		"system:file:upload":          "Upload File",
		"system:config:auth-provider": "Third-party Login",
		"system:config:storage":       "Storage Config",
		"system:cache:manage":         "Cache Management",
		"system:plugin:query":         "Query Plugin",
		// Unknown segments fall back to the raw code, same as Name.
		"unknown:thing:create": "unknown:thing:create",
		"example:ping":         "example:ping",
	}
	for code, want := range cases {
		if got := menuperm.NameEn(menuperm.Route{}, code); got != want {
			t.Errorf("%s: got %q want %q", code, got, want)
		}
	}
}

func TestBuildDeduplicatesCodesAndAssignsSort(t *testing.T) {
	menuIDs := map[string]uint64{"system_user": 11}
	routes := []menuperm.Route{
		{PluginID: "system", Builtin: true, Codes: []string{"system:user:query"}, Method: "GET", Path: "/api/system/users"},
		{PluginID: "system", Builtin: true, Codes: []string{"system:user:query"}, Method: "GET", Path: "/api/system/users/:id"},
		{PluginID: "system", Builtin: true, Codes: []string{"system:user:create"}, Method: "POST", Path: "/api/system/users"},
	}

	buttons, unresolved := menuperm.Build(routes, menuIDs)
	if len(unresolved) != 0 {
		t.Fatalf("unexpected unresolved: %+v", unresolved)
	}
	if len(buttons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(buttons))
	}
	if buttons[0].Sort != 10 || buttons[1].Sort != 20 {
		t.Fatalf("unexpected sort values: %d %d", buttons[0].Sort, buttons[1].Sort)
	}
	if buttons[0].Path != "/api/system/users" {
		t.Fatalf("first declaration should win, got %q", buttons[0].Path)
	}
	if !buttons[1].Sensitive || buttons[0].Sensitive {
		t.Fatalf("sensitivity should follow the method")
	}
}

func TestBuildReportsUnresolvedParent(t *testing.T) {
	routes := []menuperm.Route{
		{PluginID: "system", Builtin: true, Codes: []string{"system:nothing:query"}, Method: "GET"},
		{PluginID: "example", Codes: []string{"example:ping"}, Method: "GET"},
	}
	buttons, unresolved := menuperm.Build(routes, map[string]uint64{})
	if len(buttons) != 0 {
		t.Fatalf("expected no buttons, got %d", len(buttons))
	}
	if len(unresolved) != 2 {
		t.Fatalf("expected 2 unresolved, got %d", len(unresolved))
	}
	if !unresolved[0].Builtin || unresolved[1].Builtin {
		t.Fatalf("builtin flag not propagated: %+v", unresolved)
	}
}
