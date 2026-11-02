# Plugin Development Guide

**English** | [中文](PLUGIN_CN.md)

Backend business features are integrated as **compile-time plugins**. A plugin only declares what it contributes; assembly, ordering, switching and migrations are all done by the kernel and the platform layer. This document explains how to write a plugin, and what you must self-check once it is written.

Related docs: [CONVENTIONS.md](CONVENTIONS.md) (layering and authorization conventions), [../../README.md](../../README.md) (quick start).

## 1. When to Make It a Plugin

Make it a plugin, instead of stuffing it into `internal/`:

- Features that go **on and off as a whole** (the example plugin, a business module).
- Features that own **their own tables** and **their own menu pages**.
- Features that need the "ship dark, effective only after an admin enables it" delivery shape.

Do not make it a plugin: infrastructure shared across plugins (that is `platform/`), the framework contract itself (that is `kernel/`), and utilities that every plugin needs to import directly (put them in `internal/pkg/`).

## 2. Directory Structure

```
plugins/<id>/
  plugin.go              Meta() + Routes(), optional Migrations()
  migrations/
    001_init.sql         migration file, append-only
  <other implementation files>.go
```

`<id>` must match `^[a-z][a-z0-9]{2,15}$` (`plugin.ValidID`). It is at the same time the directory name, the table name prefix, the configuration namespace and the value of `sys_menu.plugin_id`.

## 3. Contract

The minimal contract has only two methods:

```go
type Plugin interface {
    Meta() Meta
    Routes() []Route
}
```

Implement the optional `Migrator` capability (`Migrations() []Migration`) only when you need to create tables — capability interfaces are collected by type assertion, so a plugin picks only the parts it needs.

```go
type Plugin struct{}

func (Plugin) Meta() plugin.Meta {
    return plugin.Meta{
        ID: "example", Name: "Example Plugin", Version: "0.1.0",
        DependsOn: []string{"system"},   // dependent plugins must be initialized first
        Order:     10,                   // tiebreaker for ordering within the same layer, no semantics
        DropTables: []string{"sys_example_item"}, // tables to DROP on uninstall
        Menus:     []plugin.Menu{{Code: "example_page", Name: "Example Plugin", /* see section 6 */}},
        Builtin:   false,                // true = cannot be disabled
    }
}
```

`DropTables` is used only when a plugin is **uninstalled** (see section 7). Migrations are append-only, so which tables were created cannot be inferred from them; a plugin that creates tables must declare them itself. Uninstall without declaring them and the tables stay in the database as orphans. Names must be plain identifiers (`^[a-z][a-z0-9_]{0,62}$`); schema-qualified, quoted or uppercase names are all rejected and not a single DDL statement is executed.

Among the `Route` fields, apart from `Method/Path/Handler` which map one-to-one to go-zero, everything else is handed to kernel middleware:

| Field | Purpose |
|------|------|
| `Public` | No authentication (login, health check, captcha). Localization and rate limiting still apply |
| `Permission` | A single permission code; empty means "signed in is enough" |
| `AnyOf` | Allow if any one of the permission codes matches |
| `FeatureKey` | Adds another feature-flag layer (checked after permission) |
| `Audit` | `nil` = record write operations per the default rules; `false` = skip the audit log |
| `AuditPermission` | Overrides the permission code recorded in the audit row |
| `MenuCode` | The page code the permission button mounts to; when empty it is derived from the permission code |
| `MenuName` | Overrides the auto-generated Chinese button name |

## 4. Registration

There is only one place: `plugins/all/all.go`.

```go
func All() []plugin.Plugin {
    return []plugin.Plugin{
        system.Plugin{},
        example.Plugin{},
        // scaffold:insert
    }
}
```

One import + one slice element is all it takes; `// scaffold:insert` is where the scaffolding appends automatically. **There is no hot reload**: enabling/disabling changes behavior, not the binary.

## 5. Migrations

- Location: `plugins/<id>/migrations/NNN_name.sql`, read in with `go:embed`. `NNN` is exactly three digits, so lexicographic order equals numeric order.
- **One file = one transaction**, committed together with its `sys_plugin_migration` record. A failure rolls back only this one file, and the next enable continues from it.
- **Append-only**: the SHA-256 of every applied file is recorded and compared on each run, and a modification makes enable fail (`ErrChecksumMismatch`). To fix something, fix forward with a new file.
- A file may contain multiple statements (executed without bound parameters, pgx uses the simple protocol), but **placeholders like `$1` must not appear**.
- All schema changes are serialized by a session-level `pg_advisory_lock`; `admin-api` is the only migration executor, and `jobs-runner` must start only after it has finished starting up.

## 6. Menus and Permissions

### Menus: declare them in `Meta.Menus`, do not write SQL

```go
Menus: []plugin.Menu{
    {
        Code:       "example_page",     // sys_menu.code, unique table-wide
        Name:       "Example Plugin",   // Chinese label, written on first insert
        NameEn:     "Example Plugin",   // seeded into sys_translation on first insert
        RouteName:  "ExamplePage",
        RoutePath:  "/example",
        Component:  "/example/index",
        Icon:       "lucide:box",
        Sort:       900,
        ParentCode: "",                 // empty = top level, or point at a core menu
    },
},
```

Four rules:

1. **Never write `sys_menu.id`.** The sequence allocates it, so a plugin needs no reserved range and cannot collide with another plugin. `plugin_id` is filled in from `Meta.ID`.
2. **`Code` is the unique key**, in the same namespace as core menus. A collision **fails startup or enable** (`ErrMenuCodeTaken`) instead of being swallowed by `ON CONFLICT DO NOTHING`, which used to leave a plugin "enabled" with no menu at all.
3. **Structure is re-applied, presentation is written once.** Parent, type, route and icon follow the declaration; `Name` / `NameEn` / `Sort` / `Hidden` / `KeepAlive` are only written on the first insert, so an administrator's edits survive.
4. **The declaration is the whole truth.** Drop a `Code` from `Menus` and the row is soft deleted on the next enable (the sidebar entry disappears) while its `sys_role_menu` grants are kept; add it back and the same row is revived with the same id and grants. Menus created by hand in the admin UI always have an empty `plugin_id`, so they never fall into a plugin's retirement scope.

`ParentCode` may point at a core menu or at another plugin's menu, so plugins can hang off the core tree. Pointing at a code that does not exist raises `ErrMenuParentMissing` — a typo never turns into a top-level menu.

Why not in a migration: a migration runs once and is then frozen by its checksum, so a wrong id is wrong forever (the only fix is another migration, while the old row still holds the slot). A declaration is replayed on every enable, so fixing the code fixes the row.

> Legacy: `plugins/example/migrations/001_init.sql` still contains an INSERT with a hard-coded `id 1000`, kept byte for byte so its checksum stays valid. The declared sync takes that row over (same code, same plugin_id) and keeps its id and grants. Do not copy it in a new plugin.

### Migrating from the old style

If an existing plugin still inserts its menu from a migration, the route depends on the plugin's state in the **target** database:

- **Never enabled there** (no row in `sys_plugin_migration`): edit `001` directly — delete the menu INSERT and move the menu into `Meta.Menus`. With no checksum on record, editing the file is safe.
- **Already enabled**: not a single byte of `001` may change. Just add the **same code** to `Meta.Menus` and enable once — the sync takes the existing row over, keeping its id and grants. The INSERT in the migration can stay; it only ever runs on first install.

Either way, ship the declaration first and confirm the sidebar looks right before touching the migration. Doing it the other way round leaves the menu missing for one release.

### Permissions: generated from `Route`, never seeded

Permission codes are generated by the startup sync from `Route.Permission` / `AnyOf` and mounted onto the page named by `MenuCode`:

```go
{
    Method: http.MethodGet, Path: "/api/example/ping",
    Permission: "example:ping",
    MenuCode:   "example_page",   // must be written explicitly when derivation fails
    MenuName:   "Connectivity check",
    Handler:    pingHandler,
}
```

Derivation rule: drop the last segment of the permission code as the action, turn hyphens in what remains into underscores, and take the first actually existing non-button menu from longest to shortest. The only candidate for `example:ping` is `example`, while the page code is `example_page`, so it must be declared explicitly.

**A core plugin whose permission codes cannot be resolved to a page makes startup fail** (that is a programming error); **a regular plugin's are skipped and logged**, waiting for its menu to be created and filled in at enable time.

## 7. Enable and Disable

| Action | What happens |
|------|----------|
| Compiled in | Inserted into `sys_plugin` with `enabled = is_builtin`, i.e. **disabled by default** |
| Enable | Run pending migrations → sync declared menus → sync permission buttons → flip `enabled = true` → reload the route gate and Casbin |
| Disable | Only flips the switch. No DDL, no data deletion, no button retirement |

The effect of disabling is that **three layers disappear at once**: routes return 404 (byte-for-byte identical to a path that does not exist), menus are trimmed at the SQL layer, and permission codes disappear from `/api/auth/codes` and Casbin. Re-enabling is immediate and lossless.

`system` is builtin and is force-enabled in three places: catalog sync, the in-memory snapshot, and `UPDATE ... WHERE is_builtin = FALSE`. So `/api/health` and `/api/ready` never return 404.

**Multiple replicas**: each replica has its own in-memory snapshot. After startup, `admin-api` runs a 5-second watcher (`plugincenter.Watch`) that re-reads `sys_plugin` periodically, so a switch flipped on replica A takes effect on replica B at most one period later. Redis subscription is not used because go-zero's redis client has no subscriber, and polling a table with only a dozen rows is a better trade than one more connection plus reconnect/lost-message handling; the cost is that during that one period B still allows requests according to the old state.

## 7.1 Uninstall

Uninstall is a **destructive** operation, strictly separate from disable: disabling just turns the light off, uninstalling tears the light out.

| Precondition | Description |
|----------|------|
| Not builtin | `system` / `plugincenter` cannot be uninstalled |
| Disabled | If not disabled, returns 409 `errors.plugin.enabled_cannot_uninstall` directly |
| No enabled dependents | Otherwise 409 `errors.plugin.dependency_in_use` |

Once satisfied, the following actions complete in **a single transaction**, and a failure at any step rolls the whole thing back:

1. `DROP TABLE IF EXISTS` the tables declared in `Meta.DropTables` (no CASCADE; an error if another plugin's foreign key references them)
2. Delete the plugin's migration records from `sys_plugin_migration`
3. Delete the `sys_role_menu` → `sys_menu` rows with `plugin_id = <id>` (pages and buttons together)
4. Delete the catalog row from `sys_plugin`

The plugin is still compiled into the binary, so the catalog sync at the next startup inserts it back as **disabled**; enabling it again re-runs the migrations from `001`, which equals a fresh install. That is the full semantics of "uninstall"; no extra "uninstalled" status flag is needed.

A missing `DropTables` declaration does not error, it only leaves orphan tables — which is why the self-check list has an item for it.

## 8. Response Conventions

Plugins **must not** import `internal/httpapi` (that is the core application's internal implementation; the only exemption is `plugins/system`, which will be removed after P6). Write your own response body and keep the envelope consistent:

```go
func pingHandler(*app.Context) http.HandlerFunc {
    return func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        _, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"pong":true}}`))
    }
}
```

When you need the database, cache or translator, take them from `*app.Context`; do not create your own connections.

## 9. Self-Check List

Before committing a new plugin:

- [ ] `plugins/all/all.go` has one more line, `go build ./...` passes
- [ ] `bash scripts/check-imports.sh` passes (plugins do not import each other)
- [ ] Migration file names are `NNN_name.sql`, with no `$1`
- [ ] Menus are declared in `Meta.Menus` with a table-wide unique `Code`; no `sys_menu.id` in a migration
- [ ] If tables were created, their names are in `Meta.DropTables`
- [ ] No permission codes seeded by hand
- [ ] After `go run ./cmd/gen-api`, `api/openapi/openapi.json` has the corresponding paths
- [ ] The route baseline `internal/kernel/testdata/routes.golden` is updated (including the count on the first line)
- [ ] On a fresh database, enable once, disable once, enable again; menus and permission codes are as expected
- [ ] Uninstall once more, confirm tables, menus and migration records are gone, and that re-enabling gets back to the initial state

Uninstall performs a real `DROP TABLE`; the unit tests only cover identifier validation. To exercise real DDL, start a one-off database:

```bash
docker run -d --rm --name dragz-verify-pg \
  -e POSTGRES_USER=verify -e POSTGRES_PASSWORD=verify -e POSTGRES_DB=verify \
  -p 25433:5432 postgres:16-alpine
DRAGZ_VERIFY_DSN='postgres://verify:verify@127.0.0.1:25433/verify?sslmode=disable' \
  go test ./internal/store -run TestUninstallPlugin -v
docker stop dragz-verify-pg
```

When `DRAGZ_VERIFY_DSN` is not set these cases are skipped automatically, so `go test ./...` remains fully offline.

## 10. Common Errors

| Symptom | Cause |
|------|------|
| Startup reports `permission "x:y" has no owning menu` | A core route's permission code cannot be derived to a page; add `Route.MenuCode` |
| Enable fails with `ErrChecksumMismatch` | A migration file that was already applied has been modified. Add a new file, or restore the original by hand |
| Enable succeeds but the button does not appear | The page named by `Route.MenuCode` is not declared in `Meta.Menus`, or its `Code` is misspelled |
| `menu code already taken` at startup or enable | Two plugins declare the same `Code`, or it collides with a core menu |
| `declared menu parent does not exist` | `ParentCode` is a typo, or the parent belongs to a plugin that is not enabled |
| After disabling, the API returns 401 instead of 404 | The route gate was placed inside authentication. `Gate` must be outside `Authenticate` |
| The menu exists but the sidebar does not show it | The menu has `status != 1`, `visible = 0`, or its parent menu is disabled |
| Plugin buttons were "deleted" by a restart | Manually created buttons have `managed = 0` and are not retired; only synced ones are. Check whether the code was taken over by the sync |
| The table is still there after uninstall | It is not declared in `Meta.DropTables`, or the name is invalid (uppercase / schema-qualified / quoted names are rejected and the whole thing rolls back) |
| Uninstall returns 409 `enabled_cannot_uninstall` | Disable first, then uninstall; the two steps are kept separate so that "stopping the service" and "throwing data away" do not become one click |
