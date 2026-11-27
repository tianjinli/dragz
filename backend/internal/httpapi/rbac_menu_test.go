package httpapi

import "testing"

func TestIsAllowedMenuType(t *testing.T) {
	allowed := []string{"directory", "page", "button"}
	for _, typ := range allowed {
		if !isAllowedMenuType(typ) {
			t.Fatalf("expected menu type %q to remain allowed for admin UI menus", typ)
		}
	}
	for _, typ := range []string{"", "external", "link", "BUTTON", "btn"} {
		if isAllowedMenuType(typ) {
			t.Fatalf("unexpected menu type allowed: %q", typ)
		}
	}
}
