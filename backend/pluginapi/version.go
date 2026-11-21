// Package pluginapi is the versioned host surface for independently delivered
// compile-time plugins.
//
// Delivery model: a plugin ships as a zip (manifest + backend sources +
// admin frontend pages/locales). The host assembles it into plugins/<id> and
// the admin SPA at build time, then compiles one binary. Runtime enable/disable
// stays in platform/plugincenter — there is no hot load.
//
// After assemble, plugin Go code may import only the packages listed in
// AllowedImportPrefixes (plus the standard library and third-party modules).
// Deep imports into internal/httpapi are forbidden (plugins/system is the
// sole documented exception until P6).
package pluginapi

// EngineVersion is the host plugin-engine version that pack/assemble and
// manifest.engineRange check against. Bump the major when breaking the
// Plugin / Meta / Route / app.Context / eventbus contracts that plugins use.
const EngineVersion = "1.0.0"

// DefaultEngineRange is written into new manifests when the author does not
// pin a narrower range. It accepts any 1.x host.
const DefaultEngineRange = ">=1.0.0 <2.0.0"

// AllowedImportPrefixes lists dragz-admin/backend import paths plugins are
// expected to use after they are assembled into this module. The list is
// documentation + assemble-time guidance; check-imports.sh still enforces
// the hard isolation rules.
var AllowedImportPrefixes = []string{
	"dragz-admin/backend/internal/plugin",
	"dragz-admin/backend/internal/app",
	"dragz-admin/backend/platform/eventbus",
	"dragz-admin/backend/pluginapi",
}
