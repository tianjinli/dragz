// Package migrate runs a plugin's forward-only SQL migrations.
//
// The engine is deliberately free of any internal/* import: it only knows a
// tiny Store interface, which keeps it unit-testable with a fake and keeps the
// dependency direction platform → (nothing).
//
// Guarantees:
//   - Files run in lexical version order.
//   - An already-applied file is skipped, unless its checksum changed, in
//     which case the run fails closed instead of silently diverging.
//   - One file = one transaction, together with its bookkeeping row, so a
//     partially applied plugin is always resumable from the failed file.
//   - A failure stops the run; earlier files stay applied.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// File is one migration contributed by a plugin.
type File struct {
	Version  string
	Name     string
	SQL      string
	Checksum string
}

// Source is one plugin's ordered migration set.
type Source struct {
	PluginID string
	Files    []File
}

// Store is the persistence the engine needs. *store.PluginRepository satisfies
// it, but the engine never imports that package.
type Store interface {
	// AppliedMigrations returns version → checksum for everything already
	// committed for the plugin.
	AppliedMigrations(ctx context.Context, pluginID string) (map[string]string, error)
	// ApplyMigration executes the script and records it atomically.
	ApplyMigration(ctx context.Context, pluginID, version, name, checksum, script string) error
}

// ErrChecksumMismatch means an already-applied file was edited. Migrations are
// append-only; the administrator must restore the original file or delete the
// bookkeeping row (break-glass).
var ErrChecksumMismatch = errors.New("migration checksum mismatch")

// Engine executes sources against a store.
type Engine struct {
	Store Store
}

// Run applies every pending file of src.
func (e *Engine) Run(ctx context.Context, src Source) error {
	if e.Store == nil {
		return errors.New("migrate: nil store")
	}
	files, err := sortedFiles(src)
	if err != nil {
		return err
	}

	applied, err := e.Store.AppliedMigrations(ctx, src.PluginID)
	if err != nil {
		return fmt.Errorf("load applied migrations for %s: %w", src.PluginID, err)
	}

	for _, file := range files {
		if checksum, ok := applied[file.Version]; ok {
			if checksum != file.Checksum {
				return fmt.Errorf("%w: plugin=%s version=%s (applied %s, current %s)",
					ErrChecksumMismatch, src.PluginID, file.Version, short(checksum), short(file.Checksum))
			}
			continue
		}
		if err := e.Store.ApplyMigration(ctx, src.PluginID, file.Version, file.Name, file.Checksum, file.SQL); err != nil {
			return fmt.Errorf("apply %s/%s: %w", src.PluginID, file.Version, err)
		}
	}
	return nil
}

// Pending counts the files that would run right now. It is used by the admin
// list so an operator can see the cost of enabling a plugin before doing it.
func (e *Engine) Pending(ctx context.Context, src Source) (int, error) {
	files, err := sortedFiles(src)
	if err != nil {
		return 0, err
	}
	applied, err := e.Store.AppliedMigrations(ctx, src.PluginID)
	if err != nil {
		return 0, fmt.Errorf("load applied migrations for %s: %w", src.PluginID, err)
	}
	pending := 0
	for _, file := range files {
		if _, ok := applied[file.Version]; !ok {
			pending++
		}
	}
	return pending, nil
}

// sortedFiles validates the source and returns a copy ordered by version.
// Sorting here (rather than trusting the caller) makes the execution order a
// property of the engine.
func sortedFiles(src Source) ([]File, error) {
	if src.PluginID == "" {
		return nil, errors.New("migrate: empty plugin ID")
	}
	files := make([]File, len(src.Files))
	copy(files, src.Files)

	seen := make(map[string]string, len(files))
	for _, file := range files {
		if file.Version == "" {
			return nil, fmt.Errorf("migrate: %s has a migration with an empty version", src.PluginID)
		}
		if other, dup := seen[file.Version]; dup {
			return nil, fmt.Errorf("migrate: %s declares version %q twice (%s and %s)",
				src.PluginID, file.Version, other, file.Name)
		}
		seen[file.Version] = file.Name
		if file.Checksum == "" {
			return nil, fmt.Errorf("migrate: %s/%s has an empty checksum", src.PluginID, file.Version)
		}
	}

	sort.SliceStable(files, func(i, j int) bool { return files[i].Version < files[j].Version })
	return files, nil
}

func short(checksum string) string {
	if len(checksum) <= 12 {
		return checksum
	}
	return checksum[:12]
}
