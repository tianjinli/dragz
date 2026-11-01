package httpapi

import (
	"testing"

	"dragz-admin/backend/internal/store"
)

func TestBuildRouteTreeKeepsParentVisibleWhenChildrenVisible(t *testing.T) {
	tree := buildRouteTree([]store.Menu{
		{
			ID:        10,
			ParentID:  0,
			Type:      "directory",
			Name:      "组织权限",
			RouteName: "Organization",
			RoutePath: "/organization",
			Visible:   false,
		},
		{
			ID:        12,
			ParentID:  10,
			Type:      "page",
			Name:      "部门管理",
			RouteName: "SystemDepartment",
			RoutePath: "/organization/departments",
			Component: "/system/department/index",
			Visible:   true,
		},
	}, false)

	if len(tree) != 1 {
		t.Fatalf("expected one root menu, got %d", len(tree))
	}
	if tree[0].Meta.HideInMenu {
		t.Fatal("expected parent menu to stay visible when it has visible children")
	}
	if len(tree[0].Children) != 1 {
		t.Fatalf("expected one child menu, got %d", len(tree[0].Children))
	}
	if tree[0].Children[0].Meta.HideInMenu {
		t.Fatal("expected child menu to stay visible")
	}
}
