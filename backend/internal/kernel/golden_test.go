package kernel_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"dragz-admin/backend/internal/kernel"
	"dragz-admin/backend/plugins/all"
)

// TestRoutesGolden locks the route table to the P0 baseline. Any change to
// METHOD, PATH, auth requirement, feature gate or audit attribution must be
// reflected in testdata/routes.golden together with a review note.
func TestRoutesGolden(t *testing.T) {
	registry, err := kernel.Build(all.All())
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}

	var got []string
	for _, bound := range registry.Routes() {
		route := bound.Route
		auth := "-"
		switch {
		case route.Permission != "":
			auth = route.Permission
		case len(route.AnyOf) > 0:
			auth = route.AnyOf[0]
		}
		feature := route.FeatureKey
		if feature == "" {
			feature = "-"
		}
		audit := "-"
		// GET/HEAD/OPTIONS are never audited (auditRoute no-ops), so the
		// attribution column stays "-" for them, matching the old index.
		if route.Method != "GET" && route.Method != "HEAD" && route.Method != "OPTIONS" {
			audit = kernel.AuditAttribution(route)
			if audit == "" {
				audit = "-"
			}
		}
		scope := "protected"
		if route.Public {
			scope = "public"
		}
		got = append(got, fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			strings.ToUpper(route.Method), route.Path, auth, feature, audit, scope))
	}

	want := readGolden(t, "testdata/routes.golden")
	if len(got) != len(want) {
		t.Fatalf("route count changed: got %d, golden %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("route #%d drifted:\n  golden: %s\n  got:    %s", i+1, want[i], got[i])
		}
	}
}

func readGolden(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
