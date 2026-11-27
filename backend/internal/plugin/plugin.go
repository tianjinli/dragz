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

// Menu is a declarative menu contribution.
//
// The platform owns the sys_menu row: it assigns the id, resolves ParentCode
// to a parent id, and re-applies the structure on every boot and every enable.
// A plugin therefore never writes sys_menu.id, and no two plugins can collide
// on one.
//
// Menus are declared here rather than seeded from a migration for the same
// reason permission buttons moved to Route.Permission: a migration runs once
// and is then frozen by its checksum, so a mistake inside it — a clashing id,
// a wrong component path — is permanent. A declaration is re-applied, which
// makes it correctable.
type Menu struct {
	// Code is sys_menu.code and the stable business key of the menu. It must
	// be unique across the whole table, core menus included. A collision with
	// another owner fails the sync loudly; it is never skipped silently.
	Code string
	// ParentCode is the Code of another menu, core or plugin-owned.
	// Empty means a top-level menu.
	ParentCode string
	// Type is "page" (default when empty) or "directory".
	Type string
	// Name is the label seeded into sys_menu.name on first insert. An
	// administrator rename is never overwritten by a later sync.
	Name string
	// NameEn seeds sys_translation as menu.<Code> on first insert, absent-on-
	// conflict, so an edited translation survives too.
	NameEn string

	// The remaining fields describe the route the menu points at.
	RouteName string
	RoutePath string
	Component string
	Redirect  string
	Icon      string
	// Sort is the first-insert ordinal within the parent. Like Name, it is an
	// initial value; an administrator's reorder is not clobbered.
	Sort int
}

// Meta describes a plugin. ID doubles as directory name, table prefix and
// config namespace; it must match ^[a-z][a-z0-9]{2,15}$.
type Meta struct {
	ID          string
	Name        string
	Version     string
	Description string
	// Menus are the plugin's sidebar entries. Their ids are assigned by the
	// platform, so declaring a menu cannot collide with another plugin. The
	// plugin_id column is set from ID automatically.
	Menus []Menu
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
	// Platform restricts the route to a platform operator or a super administrator.
	// It does not create a permission button.
	Platform bool
	// OpenScopes lists API-token scopes that may call this route. The guard
	// allows the request when the token includes any listed scope, the blanket
	// scope "open", or "*". Empty means the route is not on the open API.
	// JWT callers have no token scopes and are rejected. Like Platform, this
	// does not create a permission button.
	OpenScopes []string
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
	// RateLimit is the requests-per-minute budget for this route. 0 means the
	// default for its class (see defaultRateLimit). Set a smaller value on
	// endpoints that cost real work (login, signup, invite lookup) and a larger
	// one on dashboards that poll.
	RateLimit int
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
//	admin : Localize → HostTenant → Gate → Authenticate → Tenant → Audit → Guard → handler
//	public: Localize → HostTenant → Gate → handler
//
// Gate sits OUTSIDE Authenticate on purpose: a route belonging to a disabled
// plugin must be indistinguishable from a route that does not exist, for
// anonymous and authenticated callers alike. It sits INSIDE Localize so the
// 404 body is translated like every other response. HostTenant sits between
// them so a public login page and a protected call both see the host tenant,
// and a disabled plugin still answers 404 before authentication.
type Hooks struct {
	Localize func(next http.HandlerFunc) http.HandlerFunc
	// HostTenant resolves the request Host before Gate. Nil skips resolution.
	HostTenant   func(next http.HandlerFunc) http.HandlerFunc
	Authenticate func(next http.HandlerFunc) http.HandlerFunc
	// Tenant resolves the caller's membership after authentication and stores
	// the tenant id and platform-operator flag on the request context.
	Tenant func(next http.HandlerFunc) http.HandlerFunc
	Audit  func(route Route, next http.HandlerFunc) http.HandlerFunc
	Guard  func(route Route, next http.HandlerFunc) http.HandlerFunc
	// RateLimit enforces the route's per-minute budget. It runs after Authenticate
	// and Tenant so the caller's identity (user + tenant) is available, and
	// before Audit so a throttled request does not fill the audit log. Nil
	// disables limiting.
	RateLimit func(route Route, next http.HandlerFunc) http.HandlerFunc
	// Gate answers 404 when the owning plugin is disabled. Nil means every
	// plugin is treated as enabled.
	Gate func(pluginID string, next http.HandlerFunc) http.HandlerFunc
}
