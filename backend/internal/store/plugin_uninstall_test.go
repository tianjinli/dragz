package store

import (
	"context"
	"strings"
	"testing"
)

// UninstallPlugin interpolates table names into DROP TABLE because SQL cannot
// bind identifiers, so the validator is the only thing standing between a bad
// plugin declaration and arbitrary DDL. It must reject before touching the
// database: a nil *DB makes any early escape obvious.
func TestUninstallPluginRejectsUnsafeIdentifiers(t *testing.T) {
	repo := &PluginRepository{}

	tests := []struct {
		name       string
		pluginID   string
		dropTables []string
		wantSubstr string
	}{
		{
			name:       "plugin id with a quote",
			pluginID:   `demo"; DROP TABLE users; --`,
			wantSubstr: "invalid plugin id",
		},
		{
			name:       "table name with a quote",
			pluginID:   "demo",
			dropTables: []string{`demo"; DROP TABLE users; --`},
			wantSubstr: "invalid table name",
		},
		{
			name:       "schema-qualified table",
			pluginID:   "demo",
			dropTables: []string{"public.demo"},
			wantSubstr: "invalid table name",
		},
		{
			name:       "uppercase table",
			pluginID:   "demo",
			dropTables: []string{"Demo"},
			wantSubstr: "invalid table name",
		},
		{
			name:       "empty table name",
			pluginID:   "demo",
			dropTables: []string{""},
			wantSubstr: "invalid table name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.UninstallPlugin(context.Background(), tt.pluginID, tt.dropTables)
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSubstr) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.wantSubstr)
			}
		})
	}
}

// A plugin that owns no tables is still uninstallable, so the validator must
// accept an empty list and a well-formed name.
func TestUninstallPluginAcceptsValidNames(t *testing.T) {
	repo := &PluginRepository{}
	// The database is nil, so reaching the transaction panics; that is the
	// signal that validation passed. A validation error returns instead.
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("expected the nil database to be reached, validation rejected a valid name")
		}
	}()
	_ = repo.UninstallPlugin(context.Background(), "demo", []string{"sys_demo_item"})
}
