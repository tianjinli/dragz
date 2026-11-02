package menuperm

import (
	"errors"
	"fmt"
)

// MenuNode is the neutral projection of one plugin.Menu. It mirrors the Route
// projection above so this package keeps depending on nothing but the standard
// library: store imports it, and store must not reach back into internal/plugin.
type MenuNode struct {
	// Code is sys_menu.code, the business key the sync upserts on.
	Code string
	// ParentCode is the Code of the parent menu, empty for a top level entry.
	// It may point at a core menu, which is resolved by the store.
	ParentCode string
	// Type is normalised to "page" or "directory" by NormalizeType.
	Type      string
	Name      string
	NameEn    string
	RouteName string
	RoutePath string
	Component string
	Redirect  string
	Icon      string
	Sort      int
	Hidden    bool
	KeepAlive bool
	// PluginID owns the row. The store refuses to touch a code owned by
	// somebody else, which is what turns a clash into an error instead of a
	// silent skip.
	PluginID string
}

var (
	// ErrMenuCodeEmpty means a declared menu has no code to upsert on.
	ErrMenuCodeEmpty = errors.New("menu code is empty")
	// ErrMenuCodeDup means two declarations in one batch share a code.
	ErrMenuCodeDup = errors.New("menu code declared twice")
	// ErrMenuTypeInvalid means a menu is neither a page nor a directory.
	ErrMenuTypeInvalid = errors.New("menu type must be page or directory")
	// ErrMenuCycle means a menu ends up below itself. The startup sync would
	// otherwise write a parent_id chain that the tree query loops on.
	ErrMenuCycle = errors.New("menu parent chain forms a cycle")
)

// NormalizeType maps the empty string to "page" and leaves the two legal
// values alone.
func NormalizeType(t string) string {
	if t == "" {
		return "page"
	}
	return t
}

// OrderMenus validates a batch of declarations and returns them parent before
// child.
//
// Ordering matters because the store inserts rows without an id: a child can
// only be given a parent_id once its parent exists. A parent that is not in
// the batch is assumed to be a core menu; the store resolves it and fails with
// a readable error when it does not exist, so a typo cannot pass unnoticed.
//
// Declaration order is preserved among siblings, which keeps the result
// deterministic (plugins are already topologically sorted by dependency).
func OrderMenus(nodes []MenuNode) ([]MenuNode, error) {
	byCode := make(map[string]MenuNode, len(nodes))
	for _, node := range nodes {
		if node.Code == "" {
			return nil, ErrMenuCodeEmpty
		}
		if _, ok := byCode[node.Code]; ok {
			return nil, fmt.Errorf("%w: %q", ErrMenuCodeDup, node.Code)
		}
		switch NormalizeType(node.Type) {
		case "page", "directory":
		default:
			return nil, fmt.Errorf("%w: %q on %q", ErrMenuTypeInvalid, node.Type, node.Code)
		}
		byCode[node.Code] = node
	}

	const (
		visiting = 1
		done     = 2
	)
	state := make(map[string]int, len(byCode))
	ordered := make([]MenuNode, 0, len(byCode))

	var visit func(code string) error
	visit = func(code string) error {
		switch state[code] {
		case done:
			return nil
		case visiting:
			return fmt.Errorf("%w: %q", ErrMenuCycle, code)
		}
		state[code] = visiting
		node := byCode[code]
		if parent := node.ParentCode; parent != "" {
			if _, inBatch := byCode[parent]; inBatch {
				if err := visit(parent); err != nil {
					return err
				}
			}
		}
		state[code] = done
		ordered = append(ordered, node)
		return nil
	}

	for _, node := range nodes {
		if err := visit(node.Code); err != nil {
			return nil, err
		}
	}
	return ordered, nil
}
