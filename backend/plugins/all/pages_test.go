package all_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dragz-admin/backend/plugins/all"
)

// TestPluginMenuComponentsMapped ensures every Meta.Menus Component declared by
// a compiled-in plugin has a matching key in frontend page-map.tsx.
func TestPluginMenuComponentsMapped(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	pageMapPath := filepath.Join(repoRoot, "frontend", "admin", "src", "router", "page-map.tsx")
	raw, err := os.ReadFile(pageMapPath)
	if err != nil {
		t.Fatalf("read page-map: %v", err)
	}
	pageMap := string(raw)

	for _, p := range all.All() {
		meta := p.Meta()
		for _, menu := range meta.Menus {
			if menu.Component == "" {
				continue
			}
			needle := "'" + menu.Component + "'"
			if !strings.Contains(pageMap, needle) {
				t.Errorf("plugin %q menu %q component %q missing from %s", meta.ID, menu.Code, menu.Component, pageMapPath)
			}
		}
	}
}
