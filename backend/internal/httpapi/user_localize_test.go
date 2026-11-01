package httpapi

import (
	"net/http/httptest"
	"testing"

	"dragz-admin/backend/internal/store"
)

// A list row as the store builds it: the aggregated source names plus the codes
// those names belong to.
func sampleUserRow() store.UserListItem {
	return store.UserListItem{
		ID:             7,
		Username:       "zhangsan",
		DisplayName:    "张三",
		Email:          "zhangsan@example.com",
		DepartmentName: "总部",
		DepartmentCode: "root",
		RoleNames:      "超级管理员, 审计员",
		PositionNames:  "工程师",
		Roles: []store.NamedRef{
			{Code: "super_admin", Name: "超级管理员"},
			{Code: "auditor", Name: "审计员"},
		},
		Positions: []store.NamedRef{{Code: "engineer", Name: "工程师"}},
	}
}

func TestLocalizeUserRowsRewritesPresetNames(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/system/users", nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")

	loader := &fakeTranslationLoader{values: map[string]string{
		"en|dept.root":        "Headquarters",
		"en|role.super_admin": "Super Administrator",
		"en|role.auditor":     "Auditor",
		"en|post.engineer":    "Engineer",
	}}
	row := localizeUserRows(request, loader, []store.UserListItem{sampleUserRow()})[0]

	if row.DepartmentName != "Headquarters" {
		t.Fatalf("department = %q, want the translated label", row.DepartmentName)
	}
	// The aggregated label keeps its order and separator, with each name
	// swapped independently.
	if row.RoleNames != "Super Administrator, Auditor" {
		t.Fatalf("roles = %q, want both names translated", row.RoleNames)
	}
	if row.PositionNames != "Engineer" {
		t.Fatalf("positions = %q, want the translated label", row.PositionNames)
	}
	// The user's own data is not preset text and must survive untouched.
	if row.DisplayName != "张三" || row.Username != "zhangsan" || row.Email != "zhangsan@example.com" {
		t.Fatalf("user's own fields changed: %+v", row)
	}
}

func TestLocalizeUserRowsFallsBackPerName(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/system/users", nil)
	request.Header.Set("Accept-Language", "en")

	// A role an administrator created has no preset translation.
	loader := &fakeTranslationLoader{values: map[string]string{
		"en|dept.root":        "Headquarters",
		"en|role.super_admin": "Super Administrator",
	}}
	row := localizeUserRows(request, loader, []store.UserListItem{sampleUserRow()})[0]

	if row.RoleNames != "Super Administrator, 审计员" {
		t.Fatalf("roles = %q, want the untranslated name kept as the source", row.RoleNames)
	}
}

func TestLocalizeUserRowsSkipsTheSourceLanguage(t *testing.T) {
	// No Accept-Language: Chinese is stored inline, so the helper must not
	// reach the database at all.
	request := httptest.NewRequest("GET", "/api/system/users", nil)
	loader := &fakeTranslationLoader{values: map[string]string{
		"zh|dept.root": "不应使用",
	}}
	row := localizeUserRows(request, loader, []store.UserListItem{sampleUserRow()})[0]

	if row.DepartmentName != "总部" || row.RoleNames != "超级管理员, 审计员" {
		t.Fatalf("row was rewritten in the source language: %+v", row)
	}
	if len(loader.asked) != 0 {
		t.Fatalf("loader was queried for %v in the source language", loader.asked)
	}
}

func TestLocalizeUserRowsKeepsAggregatedTextWithoutRefs(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/system/users", nil)
	request.Header.Set("Accept-Language", "en")

	loader := &fakeTranslationLoader{values: map[string]string{
		"en|dept.root": "Headquarters",
	}}
	item := sampleUserRow()
	// A row whose codes could not be parsed must keep its label instead of
	// collapsing to an empty cell.
	item.Roles = nil
	item.Positions = nil
	row := localizeUserRows(request, loader, []store.UserListItem{item})[0]

	if row.RoleNames != "超级管理员, 审计员" {
		t.Fatalf("roles = %q, want the aggregated text untouched", row.RoleNames)
	}
	if row.PositionNames != "工程师" {
		t.Fatalf("positions = %q, want the aggregated text untouched", row.PositionNames)
	}
}

func TestLocalizeUserRowsAsksForEachKeyOnce(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/system/users", nil)
	request.Header.Set("Accept-Language", "en")

	loader := &fakeTranslationLoader{values: map[string]string{}}
	// Two users in the same department and with the same role: the lookup must
	// not repeat a key per row.
	items := []store.UserListItem{sampleUserRow(), sampleUserRow()}
	localizeUserRows(request, loader, items)

	if len(loader.asked) != 4 {
		t.Fatalf("loader was asked for %v, want 4 distinct keys", loader.asked)
	}
}

func TestLocalizeRefSkipsAnEmptyReference(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/auth/profile", nil)
	request.Header.Set("Accept-Language", "en")

	loader := &fakeTranslationLoader{values: map[string]string{"en|dept.root": "Headquarters"}}
	// A user without a department has nothing to look up.
	ref := localizeRef(request, loader, "dept.", store.NamedRef{})

	if ref.Code != "" || ref.Name != "" {
		t.Fatalf("empty ref became %+v", ref)
	}
	if len(loader.asked) != 0 {
		t.Fatalf("loader was queried for %v with no reference", loader.asked)
	}
}

func TestLocalizeRefRewritesOneDepartment(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/auth/profile", nil)
	request.Header.Set("Accept-Language", "en")

	loader := &fakeTranslationLoader{values: map[string]string{"en|dept.root": "Headquarters"}}
	ref := localizeRef(request, loader, "dept.", store.NamedRef{Code: "root", Name: "总部"})

	if ref.Name != "Headquarters" {
		t.Fatalf("department = %q, want the translated label", ref.Name)
	}
	if ref.Code != "root" {
		t.Fatalf("code changed to %q", ref.Code)
	}
}
