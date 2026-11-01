// Package plugin defines the compile-time plugin contract.
//
// A plugin declares what it contributes (routes today; migrations, seeds,
// jobs, events in later phases) and the kernel assembles it. Plugins never
// call the framework: they only declare.
package plugin

import (
	"net/http"
	"regexp"

	"dragz-admin/backend/internal/app"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9]{2,15}$`)

// ValidID reports whether id is an acceptable plugin ID. The kernel, the
// plugin store and the HTTP layer must agree on exactly one definition.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Meta describes a plugin. ID doubles as directory name, table prefix and
// config namespace; it must match ^[a-z][a-z0-9]{2,15}$.
type Meta struct {
	ID          string
	Name        string
	Version     string
	Description string
	// DependsOn lists plugin IDs that must be initialized first.
	DependsOn []string
	// Order is a same-level sort tiebreaker only; it carries no semantics.
	Order int
	// DropTables lists the tables this plugin owns, as created by its
	// migrations. Uninstall drops them (and the plugin's menus, migration
	// bookkeeping and catalog row) so the plugin can be installed again from
	// scratch. Migrations are forward-only, so this list cannot be derived
	// from them: a plugin that creates tables must declare them here.
	//
	// Names are plain identifiers (no schema, no quotes); anything else is
	// rejected before any DDL runs.
	DropTables []string
	// Builtin plugins (system, plugincenter) cannot be disabled or uninstalled.
	Builtin bool
}

// Route is a declarative route contribution. Method/Path/Handler map 1:1 to
// go-zero rest.Route; every other field is consumed by kernel middleware.
type Route struct {
	Method string
	Path   string
	// Public = true means the route needs no authentication (login, health,
	// captcha). Public routes still get localization and rate limiting.
	Public bool
	// Permission is the single permission code required. Empty = login only.
	Permission string
	// AnyOf grants access when ANY of the codes matches (see requireAnyPermission).
	AnyOf []string
	// FeatureKey gates the route behind a plugin/tenant feature switch
	// (resolved AFTER permission, mirroring the current requireFeature nesting).
	FeatureKey string
	// Audit: nil = audit mutating requests (current behavior); false = skip.
	Audit *bool
	// AuditPermission overrides the permission code recorded in audit rows.
	// Empty falls back to Permission, then AnyOf[0].
	AuditPermission string
	// MenuCode is the sys_menu.code of the page the permission button hangs
	// under. Empty means "derive it from the permission code", which is right
	// for every core route; set it only when the page code cannot be derived
	// (e.g. a plugin whose menu code does not share the permission prefix).
	MenuCode string
	// MenuName overrides the generated Chinese label of the permission button.
	// Empty means "derive it from the permission code" (see internal/menuperm).
	MenuName string
	// Handler receives the shared app.Context at registration time and
	// returns the endpoint handler. Existing handlers already have this
	// exact signature, so they can be referenced directly.
	Handler func(ctx *app.Context) http.HandlerFunc
}

// Plugin is the minimal contract every backend plugin implements.
// Capability interfaces (Migrator, Seeder, Jobber, Subscriber) are collected
// via type assertions, so a plugin opts in only to what it needs.
type Plugin interface {
	Meta() Meta
	Routes() []Route
}

// Migration is one forward-only SQL file contributed by a plugin.
//
// Rules enforced by platform/migrate:
//   - Version is unique within a plugin and sorts lexically ("001", "002").
//   - An applied file is immutable: its checksum is recorded and compared on
//     every run, and a mismatch fails closed.
//   - The SQL may contain multiple statements because it is executed with no
//     bind parameters (pgx then uses the simple protocol), but it must not
//     contain $1-style placeholders.
type Migration struct {
	Version string
	Name    string
	SQL     string
}

// MigrationSource groups one plugin's migrations for the engine.
type MigrationSource struct {
	PluginID   string
	Migrations []Migration
}

// Migrator is the optional capability that contributes SQL migrations. The
// engine runs them when the plugin is enabled, and skips files whose version
// is already recorded.
type Migrator interface {
	Migrations() []Migration
}

// Hooks carries the middleware implementations the kernel wires into the two
// request chains. The kernel owns the ORDER; implementations live in the
// platform layer (currently internal/httpapi until P6).
//
//	admin : Localize → Gate → Authenticate → Audit → Guard → handler
//	public: Localize → Gate → handler
//
// Gate sits OUTSIDE Authenticate on purpose: a route belonging to a disabled
// plugin must be indistinguishable from a route that does not exist, for
// anonymous and authenticated callers alike. It sits INSIDE Localize so the
// 404 body is translated like every other response.
type Hooks struct {
	Localize     func(next http.HandlerFunc) http.HandlerFunc
	Authenticate func(next http.HandlerFunc) http.HandlerFunc
	Audit        func(route Route, next http.HandlerFunc) http.HandlerFunc
	Guard        func(route Route, next http.HandlerFunc) http.HandlerFunc
	// Gate answers 404 when the owning plugin is disabled. Nil means every
	// plugin is treated as enabled.
	Gate func(pluginID string, next http.HandlerFunc) http.HandlerFunc
}
