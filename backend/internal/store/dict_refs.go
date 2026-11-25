package store

import (
	"context"
	"fmt"
	"regexp"
	"sync"
)

// DictRef declares that one column of one business table stores the values of
// one dictionary type. The reference check walks these declarations before a
// dictionary item is disabled, deleted, or re-valued, and refuses the change
// while any live row still carries the value.
//
// Declarations are code, not data: plugins (and generated modules) register
// them at init time, so the identifiers in the dynamic query below are
// compile-time constants that passed the pattern check — never user input.
// A database-side registry would trade that guarantee for nothing, because
// the same code that owns the table owns the declaration.
type DictRef struct {
	Type   string
	Table  string
	Column string
}

var (
	dictRefsMu   sync.RWMutex
	dictRefs     []DictRef
	dictRefsSeen = map[DictRef]bool{}
	dictRefIdent = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

// RegisterDictReference declares that table.column stores dictTypeCode values.
// All three identifiers must be plain snake_case names; anything else is
// refused so a typo cannot turn the reference-count query into a different
// statement. Duplicate declarations are ignored, so a plugin's init and its
// generated module may both register the same pair.
func RegisterDictReference(dictTypeCode, table, column string) error {
	ref := DictRef{Type: dictTypeCode, Table: table, Column: column}
	for _, part := range []string{ref.Type, ref.Table, ref.Column} {
		if !dictRefIdent.MatchString(part) {
			return fmt.Errorf("dict reference %s on %s.%s: invalid identifier", dictTypeCode, table, column)
		}
	}
	dictRefsMu.Lock()
	defer dictRefsMu.Unlock()
	if !dictRefsSeen[ref] {
		dictRefsSeen[ref] = true
		dictRefs = append(dictRefs, ref)
	}
	return nil
}

// dictTypeReferences returns the declared references of one dictionary type.
func dictTypeReferences(dictTypeCode string) []DictRef {
	dictRefsMu.RLock()
	defer dictRefsMu.RUnlock()
	// The registry is small (one declaration per dict-backed column), so a
	// linear scan per check is fine and avoids a second index to maintain.
	var out []DictRef
	for _, ref := range dictRefs {
		if ref.Type == dictTypeCode {
			out = append(out, ref)
		}
	}
	return out
}

// DictReference is a declared reference together with how many live rows of
// the caller's tenant still carry the checked value.
type DictReference struct {
	Table  string `json:"table"`
	Column string `json:"column"`
	Count  int64  `json:"count"`
}

// CountDictItemReferences counts live rows referencing one dictionary value,
// across every registered table of the value's type. Row-level security
// scopes each count to the caller's tenant, so a value used only by another
// tenant does not block this one. An unregistered type (all of them, until a
// module declares its columns) reports no references.
func (r *AuthRepository) CountDictItemReferences(ctx context.Context, dictTypeCode, value string) ([]DictReference, error) {
	refs := dictTypeReferences(dictTypeCode)
	if len(refs) == 0 {
		return nil, nil
	}
	out := make([]DictReference, 0, len(refs))
	for _, ref := range refs {
		// BeginTx installs the tenant session variables from ctx; the
		// identifiers were validated at registration time, so interpolating
		// them is safe and sqlc cannot express this query (dynamic table).
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		var count int64
		err = tx.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = $1 AND deleted_at IS NULL`, ref.Table, ref.Column),
			value).Scan(&count)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("count dict references on %s.%s: %w", ref.Table, ref.Column, err)
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		out = append(out, DictReference{Table: ref.Table, Column: ref.Column, Count: count})
	}
	return out, nil
}

// dictItemInUse reports whether any registered table still carries the value.
func (r *AuthRepository) dictItemInUse(ctx context.Context, dictTypeCode, value string) (bool, error) {
	refs, err := r.CountDictItemReferences(ctx, dictTypeCode, value)
	if err != nil {
		return false, err
	}
	for _, ref := range refs {
		if ref.Count > 0 {
			return true, nil
		}
	}
	return false, nil
}
