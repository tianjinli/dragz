package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeTranslationLoader stands in for the store: it records the keys the
// helper asked for and answers from a fixed table keyed by lang + entity key.
type fakeTranslationLoader struct {
	values map[string]string
	asked  []string
	err    error
}

func (f *fakeTranslationLoader) TranslationsByKey(_ context.Context, lang string, keys []string) (map[string]string, error) {
	f.asked = append(f.asked, keys...)
	if f.err != nil {
		return nil, f.err
	}
	found := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := f.values[lang+"|"+key]; ok {
			found[key] = value
		}
	}
	return found, nil
}

type roleRow struct {
	Code string
	Name string
}

// localizeRoles is the shape profileHandler uses: the code addresses the
// translation, the stored name is the fallback.
func localizeRoles(r *http.Request, loader translationLoader, roles []roleRow) []roleRow {
	return localizePreset(r, loader, roles,
		func(role roleRow) []string { return []string{"role." + role.Code} },
		func(role *roleRow, values map[string]string) {
			if s, ok := values["role."+role.Code]; ok {
				role.Name = s
			}
		})
}

func TestLocalizePresetRewritesNamesInATargetLanguage(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/auth/profile", nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")

	loader := &fakeTranslationLoader{values: map[string]string{
		"en|role.super_admin": "Super Administrator",
	}}
	roles := localizeRoles(request, loader, []roleRow{{Code: "super_admin", Name: "超级管理员"}})

	if roles[0].Name != "Super Administrator" {
		t.Fatalf("role name = %q, want the translated label", roles[0].Name)
	}
	if roles[0].Code != "super_admin" {
		t.Fatalf("code changed to %q; the permission checks read it", roles[0].Code)
	}
	if len(loader.asked) != 1 || loader.asked[0] != "role.super_admin" {
		t.Fatalf("loader was asked for %v, want exactly role.super_admin", loader.asked)
	}
}

func TestLocalizePresetLeavesTheSourceLanguageAlone(t *testing.T) {
	// No Accept-Language at all: Chinese is the stored language, so the helper
	// must not even reach the database.
	request := httptest.NewRequest("GET", "/api/auth/profile", nil)
	loader := &fakeTranslationLoader{values: map[string]string{
		"zh|role.super_admin": "不应使用",
	}}

	roles := localizeRoles(request, loader, []roleRow{{Code: "super_admin", Name: "超级管理员"}})

	if roles[0].Name != "超级管理员" {
		t.Fatalf("role name = %q, want the stored Chinese text", roles[0].Name)
	}
	if len(loader.asked) != 0 {
		t.Fatalf("loader was queried for %v in the source language", loader.asked)
	}
}

func TestLocalizePresetFallsBackWhenATranslationIsMissing(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/auth/profile", nil)
	request.Header.Set("Accept-Language", "en")

	// A role an administrator created: no preset translation exists for it.
	loader := &fakeTranslationLoader{values: map[string]string{}}
	roles := localizeRoles(request, loader, []roleRow{{Code: "qa_role", Name: "测试角色"}})

	if roles[0].Name != "测试角色" {
		t.Fatalf("role name = %q, want the stored text as the fallback", roles[0].Name)
	}
}

func TestLocalizePresetIgnoresALoaderFailure(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/auth/profile", nil)
	request.Header.Set("Accept-Language", "en")

	loader := &fakeTranslationLoader{err: errors.New("db down")}
	roles := localizeRoles(request, loader, []roleRow{{Code: "super_admin", Name: "超级管理员"}})

	// A missing translation must never turn a profile read into an error page.
	if roles[0].Name != "超级管理员" {
		t.Fatalf("role name = %q, want the stored text when the lookup fails", roles[0].Name)
	}
}
