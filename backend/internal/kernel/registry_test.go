package kernel_test

import (
	"net/http"
	"strings"
	"testing"

	"dragz-admin/backend/internal/kernel"
	"dragz-admin/backend/internal/plugin"
)

type stubPlugin struct {
	meta   plugin.Meta
	routes []plugin.Route
}

func (s stubPlugin) Meta() plugin.Meta      { return s.meta }
func (s stubPlugin) Routes() []plugin.Route { return s.routes }

// A permission code becomes exactly one sys_menu button row, so two plugins
// must never claim the same code: the startup sync would hand one plugin's
// grants to the other without any error.
func TestBuildRejectsPermissionCodeOwnedByTwoPlugins(t *testing.T) {
	_, err := kernel.Build([]plugin.Plugin{
		stubPlugin{
			meta:   plugin.Meta{ID: "alpha", Builtin: true},
			routes: []plugin.Route{{Method: http.MethodGet, Path: "/api/alpha", Permission: "alpha:query"}},
		},
		stubPlugin{
			meta:   plugin.Meta{ID: "beta"},
			routes: []plugin.Route{{Method: http.MethodGet, Path: "/api/beta", AnyOf: []string{"beta:query", "alpha:query"}}},
		},
	})
	if err == nil {
		t.Fatal("expected an error for a permission code declared by two plugins")
	}
	if !strings.Contains(err.Error(), "alpha:query") {
		t.Fatalf("error should name the offending code, got %v", err)
	}
}

// One plugin may reuse a code across several routes (list + detail), which is
// the common case for :query.
func TestBuildAllowsRepeatedCodeWithinOnePlugin(t *testing.T) {
	_, err := kernel.Build([]plugin.Plugin{
		stubPlugin{
			meta: plugin.Meta{ID: "alpha", Builtin: true},
			routes: []plugin.Route{
				{Method: http.MethodGet, Path: "/api/alpha", Permission: "alpha:query"},
				{Method: http.MethodGet, Path: "/api/alpha/:id", Permission: "alpha:query"},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
