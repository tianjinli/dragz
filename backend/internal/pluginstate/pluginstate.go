// Package pluginstate is the boundary between the plugin runtime and the rest
// of the application.
//
// It exists to break an import cycle: internal/plugin imports internal/app
// (because Route.Handler takes *app.Context), so internal/app must not import
// internal/plugin. Likewise internal/store cannot import internal/plugin
// (store <- app <- plugin <- app). Everything that needs to ask "is this
// plugin enabled?" therefore depends on this package instead, which in turn
// imports nothing but the standard library.
package pluginstate

import (
	"context"
	"errors"
)

// Info is the admin-facing view of one plugin: its declaration merged with its
// persisted state.
type Info struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	DependsOn   []string `json:"dependsOn"`
	// Builtin plugins cannot be disabled; their switch is read-only.
	Builtin bool `json:"builtin"`
	Enabled bool `json:"enabled"`
	// LastError is the reason the plugin was auto-disabled, or the reason the
	// last enable attempt failed. Empty when healthy.
	LastError string `json:"lastError"`
	// PendingMigrations is how many SQL files would run on the next enable.
	PendingMigrations int `json:"pendingMigrations"`
}

// Runtime is the read/write surface consumed by app.Context and the HTTP
// layer. Implemented by platform/plugincenter.
type Runtime interface {
	// Enabled reports whether the plugin is live. It must be cheap: it is
	// called on every request.
	Enabled(pluginID string) bool
	// Builtin reports whether the plugin is protected from being disabled.
	Builtin(pluginID string) bool
	// List returns every compiled-in plugin with its persisted state.
	List(ctx context.Context) ([]Info, error)
	// Enable runs any pending migrations, then marks the plugin enabled.
	Enable(ctx context.Context, pluginID string) error
	// Disable marks the plugin disabled. It never drops data.
	Disable(ctx context.Context, pluginID string) error
	// Uninstall removes every trace of a disabled plugin: the tables it
	// declared, its menus, its migration bookkeeping and its catalog row. The
	// plugin stays compiled in and reappears as disabled on the next startup,
	// ready to be installed again from migration 001.
	Uninstall(ctx context.Context, pluginID string) error
}

// Sentinel errors mapped to HTTP responses by the handler layer.
var (
	// ErrNotFound means no compiled-in plugin has that ID.
	ErrNotFound = errors.New("plugin not found")
	// ErrBuiltin means the plugin is compiled in as a builtin and its switch is read-only.
	ErrBuiltin = errors.New("builtin plugin cannot be toggled")
	// ErrDependencyDisabled means the plugin cannot be enabled because something it depends on is off.
	ErrDependencyDisabled = errors.New("plugin dependency is disabled")
	// ErrDependencyInUse means the plugin cannot be disabled because an enabled plugin depends on it.
	ErrDependencyInUse = errors.New("plugin is required by an enabled plugin")
	// ErrMigrationFailed means a migration failed, so the plugin stays disabled.
	ErrMigrationFailed = errors.New("plugin migration failed")
	// ErrMigrationBusy means another process holds the advisory lock that serialises plugin schema changes.
	ErrMigrationBusy = errors.New("plugin migration is busy")
	// ErrPluginEnabled means the plugin must be disabled before it can be uninstalled.
	ErrPluginEnabled = errors.New("plugin must be disabled before it can be uninstalled")
)
