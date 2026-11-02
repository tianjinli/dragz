package menuperm

import (
	"errors"
	"testing"
)

func codes(nodes []MenuNode) []string {
	out := make([]string, len(nodes))
	for i, node := range nodes {
		out[i] = node.Code
	}
	return out
}

func TestOrderMenusPutsParentBeforeChild(t *testing.T) {
	nodes := []MenuNode{
		{Code: "child", ParentCode: "root"},
		{Code: "root"},
		{Code: "grandchild", ParentCode: "child"},
	}
	ordered, err := OrderMenus(nodes)
	if err != nil {
		t.Fatalf("OrderMenus: %v", err)
	}
	want := []string{"root", "child", "grandchild"}
	if got := codes(ordered); len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}
}

func TestOrderMenusKeepsDeclarationOrderAmongSiblings(t *testing.T) {
	nodes := []MenuNode{
		{Code: "a"},
		{Code: "b"},
		{Code: "c"},
	}
	ordered, err := OrderMenus(nodes)
	if err != nil {
		t.Fatalf("OrderMenus: %v", err)
	}
	got := codes(ordered)
	for i, want := range []string{"a", "b", "c"} {
		if got[i] != want {
			t.Fatalf("got %v, want stable order a,b,c", got)
		}
	}
}

// A parent outside the batch is a core menu: ordering must not reject it, the
// store resolves it and reports a missing one.
func TestOrderMenusAllowsParentOutsideBatch(t *testing.T) {
	ordered, err := OrderMenus([]MenuNode{{Code: "plugin_page", ParentCode: "system_settings"}})
	if err != nil {
		t.Fatalf("OrderMenus: %v", err)
	}
	if len(ordered) != 1 || ordered[0].Code != "plugin_page" {
		t.Fatalf("got %v", codes(ordered))
	}
}

func TestOrderMenusRejectsDuplicateCode(t *testing.T) {
	_, err := OrderMenus([]MenuNode{{Code: "dup"}, {Code: "dup"}})
	if !errors.Is(err, ErrMenuCodeDup) {
		t.Fatalf("got %v, want ErrMenuCodeDup", err)
	}
}

func TestOrderMenusRejectsEmptyCode(t *testing.T) {
	_, err := OrderMenus([]MenuNode{{Name: "no code"}})
	if !errors.Is(err, ErrMenuCodeEmpty) {
		t.Fatalf("got %v, want ErrMenuCodeEmpty", err)
	}
}

func TestOrderMenusRejectsBadType(t *testing.T) {
	_, err := OrderMenus([]MenuNode{{Code: "x", Type: "button"}})
	if !errors.Is(err, ErrMenuTypeInvalid) {
		t.Fatalf("got %v, want ErrMenuTypeInvalid", err)
	}
}

func TestOrderMenusRejectsSelfParent(t *testing.T) {
	_, err := OrderMenus([]MenuNode{{Code: "loop", ParentCode: "loop"}})
	if !errors.Is(err, ErrMenuCycle) {
		t.Fatalf("got %v, want ErrMenuCycle", err)
	}
}

func TestOrderMenusRejectsCycle(t *testing.T) {
	nodes := []MenuNode{
		{Code: "a", ParentCode: "b"},
		{Code: "b", ParentCode: "a"},
	}
	_, err := OrderMenus(nodes)
	if !errors.Is(err, ErrMenuCycle) {
		t.Fatalf("got %v, want ErrMenuCycle", err)
	}
}

func TestNormalizeTypeDefaultsToPage(t *testing.T) {
	if got := NormalizeType(""); got != "page" {
		t.Fatalf("NormalizeType(\"\") = %q, want page", got)
	}
	if got := NormalizeType("directory"); got != "directory" {
		t.Fatalf("NormalizeType(directory) = %q", got)
	}
}
