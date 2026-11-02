// Package example is the reference plugin: the smallest thing that still
// exercises every part of the plugin pipeline (routes, a SQL migration, a menu,
// a permission) so the plugin center can be verified end to end.
//
// It ships DISABLED. Nothing below exists in the database until an
// administrator enables it in 系统设置 → 插件中心, and disabling it again leaves
// every table and row in place.
package example

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/plugin"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Plugin implements plugin.Plugin and plugin.Migrator.
type Plugin struct{}

func (Plugin) Meta() plugin.Meta {
	return plugin.Meta{
		ID:          "example",
		Name:        "示例插件",
		Version:     "0.1.0",
		Description: "参考实现：一条路由 + 一个迁移 + 一个菜单，用于验证插件开关链路",
		DependsOn:   []string{"system"},
		Order:       10,
		Builtin:     false,
		// The sidebar entry. Declared here rather than seeded from 001_init.sql
		// because the platform assigns the row id: a plugin never writes
		// sys_menu.id, so two plugins cannot collide on one.
		//
		// 001_init.sql still contains the original INSERT with a hard-coded id
		// 1000. It is kept byte for byte (an applied migration is immutable and
		// checksum-verified) and this declaration takes the row over, keeping
		// its id and its grants.
		Menus: []plugin.Menu{
			{
				Code:      "example_page",
				Name:      "示例插件",
				NameEn:    "Example Plugin",
				RouteName: "ExamplePage",
				RoutePath: "/example",
				Component: "/example/index",
				Icon:      "lucide:box",
				Sort:      900,
			},
		},
		// The one table 001_init.sql creates. Uninstall drops exactly this, so
		// re-enabling the plugin replays the migration against a clean slate.
		DropTables: []string{"sys_example_item"},
	}
}

func (Plugin) Routes() []plugin.Route {
	return []plugin.Route{
		{
			Method:     http.MethodGet,
			Path:       "/api/example/ping",
			Permission: "example:ping",
			// The permission button hangs under the plugin's own page. The
			// derived candidate would be "example", which is not a menu code,
			// so the page is declared explicitly.
			MenuCode: "example_page",
			MenuName: "连通性检查",
			Handler:  pingHandler,
		},
	}
}

// Migrations reads the embedded files. Filenames are "001_init.sql": the digits
// before the first underscore are the version, the rest is the name.
//
// A malformed name is a build-time bug, so it panics instead of silently
// shipping a plugin that never migrates.
func (Plugin) Migrations() []plugin.Migration {
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		panic("example: read embedded migrations: " + err.Error())
	}

	migrations := make([]plugin.Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), ".sql")
		version, name, found := strings.Cut(base, "_")
		if !found || version == "" {
			panic("example: migration " + entry.Name() + " must be named <version>_<name>.sql")
		}
		body, err := migrationFS.ReadFile(path.Join("migrations", entry.Name()))
		if err != nil {
			panic("example: read migration " + entry.Name() + ": " + err.Error())
		}
		migrations = append(migrations, plugin.Migration{
			Version: version,
			Name:    name,
			SQL:     string(body),
		})
	}
	return migrations
}

// pingHandler answers with the standard envelope. A plugin cannot reach
// internal/httpapi helpers, which is the point: it owns its own response.
func pingHandler(*app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"pong":true,"plugin":"example"}}`))
	}
}
