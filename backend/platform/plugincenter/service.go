// Package plugincenter owns the runtime state of every compiled-in plugin:
// the catalog rows in sys_plugin, the in-memory snapshot the route gate reads,
// and the enable/disable workflow including its migrations.
//
// It is the only package allowed to combine the plugin contract
// (internal/plugin) with persistence (internal/store) and the migration engine
// (platform/migrate).
package plugincenter

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/kernel"
	"dragz-admin/backend/internal/menuperm"
	pkgi18n "dragz-admin/backend/internal/pkg/i18n"
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/internal/pluginstate"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/platform/migrate"
)

const (
	// startupLockTimeout is generous: a cold start may have to run many
	// migrations while another replica finishes its own.
	startupLockTimeout = 60 * time.Second
	// toggleLockTimeout is what an HTTP request is willing to wait.
	toggleLockTimeout = 10 * time.Second
)

// versionPattern keeps migration versions fixed-width so lexical order is
// numeric order. 999 migrations per plugin is far beyond what a plugin needs.
var versionPattern = regexp.MustCompile(`^\d{3}$`)

// Service implements pluginstate.Runtime.
type Service struct {
	repo    store.PluginStore
	engine  *migrate.Engine
	metas   []plugin.Meta
	order   []string
	byID    map[string]plugin.Meta
	sources map[string]migrate.Source
	// routes is the declared route table projected into permission buttons.
	// It is static: the compiled binary defines it.
	routes []menuperm.Route

	state atomic.Pointer[snapshot]
}

type snapshot struct {
	enabled map[string]bool
	builtin map[string]bool
}

var _ pluginstate.Runtime = (*Service)(nil)

// New validates the catalog and migration sources. It does not touch the
// database: RunStartup does that.
func New(repo store.PluginStore, metas []plugin.Meta, sources []plugin.MigrationSource, bounds []kernel.Bound) (*Service, error) {
	if repo == nil {
		return nil, errors.New("plugincenter: nil repository")
	}

	builtin := make(map[string]bool, len(metas))
	for _, meta := range metas {
		builtin[meta.ID] = meta.Builtin
	}

	service := &Service{
		repo:    repo,
		engine:  &migrate.Engine{Store: repo},
		routes:  permissionRoutes(bounds, builtin),
		metas:   make([]plugin.Meta, 0, len(metas)),
		order:   make([]string, 0, len(metas)),
		byID:    make(map[string]plugin.Meta, len(metas)),
		sources: make(map[string]migrate.Source, len(sources)),
	}

	for _, meta := range metas {
		if !plugin.ValidID(meta.ID) {
			return nil, fmt.Errorf("plugincenter: invalid plugin ID %q", meta.ID)
		}
		if _, dup := service.byID[meta.ID]; dup {
			return nil, fmt.Errorf("plugincenter: duplicate plugin ID %q", meta.ID)
		}
		service.byID[meta.ID] = meta
		service.metas = append(service.metas, meta)
		service.order = append(service.order, meta.ID)
	}

	for _, meta := range service.metas {
		for _, dep := range meta.DependsOn {
			if _, ok := service.byID[dep]; !ok {
				return nil, fmt.Errorf("plugincenter: plugin %q depends on unknown plugin %q", meta.ID, dep)
			}
		}
	}

	for _, source := range sources {
		if _, ok := service.byID[source.PluginID]; !ok {
			return nil, fmt.Errorf("plugincenter: migrations declared for unknown plugin %q", source.PluginID)
		}
		if _, dup := service.sources[source.PluginID]; dup {
			return nil, fmt.Errorf("plugincenter: duplicate migration source for %q", source.PluginID)
		}
		files := make([]migrate.File, 0, len(source.Migrations))
		seen := make(map[string]string, len(source.Migrations))
		for _, migration := range source.Migrations {
			version := strings.TrimSpace(migration.Version)
			if !versionPattern.MatchString(version) {
				return nil, fmt.Errorf("plugincenter: %s migration version %q must match ^\\d{3}$",
					source.PluginID, migration.Version)
			}
			if other, dup := seen[version]; dup {
				return nil, fmt.Errorf("plugincenter: %s declares version %q twice (%s and %s)",
					source.PluginID, version, other, migration.Name)
			}
			seen[version] = migration.Name
			if strings.TrimSpace(migration.SQL) == "" {
				return nil, fmt.Errorf("plugincenter: %s/%s has an empty migration body", source.PluginID, version)
			}
			files = append(files, migrate.File{
				Version:  version,
				Name:     strings.TrimSpace(migration.Name),
				SQL:      migration.SQL,
				Checksum: checksum(migration.SQL),
			})
		}
		service.sources[source.PluginID] = migrate.Source{PluginID: source.PluginID, Files: files}
	}

	return service, nil
}

// RunStartup brings the database and the runtime snapshot up to date.
//
// Order matters: DDL, then seeds, then the catalog, then migrations for the
// plugins that are enabled. A non-builtin plugin whose migrations fail is
// disabled and reported, but the process keeps booting; a builtin failure is
// fatal because the platform itself would be broken.
func (s *Service) RunStartup(ctx context.Context) error {
	release, err := s.repo.AcquireMigrationLock(ctx, startupLockTimeout)
	if err != nil {
		return fmt.Errorf("plugin startup: %w", err)
	}
	defer release()

	if err := s.repo.Bootstrap(ctx); err != nil {
		return err
	}
	if err := s.repo.Seed(ctx); err != nil {
		return err
	}
	if err := s.repo.SyncCatalog(ctx, s.catalogEntries()); err != nil {
		return err
	}
	if err := s.reload(ctx); err != nil {
		return err
	}

	for _, id := range s.order {
		meta := s.byID[id]
		if !s.Enabled(id) {
			continue
		}

		if missing := s.disabledDependencies(id); len(missing) > 0 {
			joined := strings.Join(missing, ", ")
			// The stored value is rendered per request (see httpapi.listPluginsHandler),
			// so the plugin's last error follows the reader's language too.
			stored := pkgi18n.StoredMessage("errors.plugin.dependency_disabled_detail", map[string]any{"Plugins": joined})
			if err := s.repo.SetPluginError(ctx, id, stored); err != nil {
				return err
			}
			if meta.Builtin {
				return fmt.Errorf("plugin %q: disabled dependency: %s", id, joined)
			}
			logx.Errorf("disabling plugin %q: disabled dependency: %s", id, joined)
			if _, err := s.repo.SetPluginEnabled(ctx, id, false); err != nil {
				return err
			}
			if err := s.reload(ctx); err != nil {
				return err
			}
			continue
		}

		source, hasMigrations := s.sources[id]
		if !hasMigrations {
			continue
		}
		if err := s.engine.Run(ctx, source); err != nil {
			if meta.Builtin {
				return fmt.Errorf("plugin %q migrations: %w", id, err)
			}
			logx.Errorf("disabling plugin %q after a failed migration: %v", id, err)
			if setErr := s.repo.SetPluginError(ctx, id, err.Error()); setErr != nil {
				logx.Errorf("record plugin %q error: %v", id, setErr)
			}
			if _, setErr := s.repo.SetPluginEnabled(ctx, id, false); setErr != nil {
				return setErr
			}
			if reloadErr := s.reload(ctx); reloadErr != nil {
				return reloadErr
			}
			continue
		}
		if err := s.repo.SetPluginError(ctx, id, ""); err != nil {
			logx.Errorf("clear plugin %q error: %v", id, err)
		}
	}

	// Runs after every migration so a plugin page created by 001_init.sql
	// already exists when its buttons are attached to it.
	if err := s.syncPermissionMenus(ctx); err != nil {
		return err
	}
	return s.reload(ctx)
}

// Enabled reports whether the plugin is live. Unknown plugins are treated as
// disabled; before the first reload everything is treated as enabled so a
// misconfigured binary does not 404 the whole API.
func (s *Service) Enabled(pluginID string) bool {
	current := s.state.Load()
	if current == nil {
		return true
	}
	if current.builtin[pluginID] {
		return true
	}
	return current.enabled[pluginID]
}

// Builtin reports whether the plugin is protected from being disabled.
func (s *Service) Builtin(pluginID string) bool {
	current := s.state.Load()
	if current == nil {
		return false
	}
	return current.builtin[pluginID]
}

// List returns every compiled-in plugin with its persisted state.
func (s *Service) List(ctx context.Context) ([]pluginstate.Info, error) {
	records, err := s.repo.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]store.PluginRecord, len(records))
	for _, record := range records {
		byID[record.ID] = record
	}

	infos := make([]pluginstate.Info, 0, len(s.metas))
	for _, meta := range s.metas {
		record := byID[meta.ID]
		dependsOn := meta.DependsOn
		if dependsOn == nil {
			dependsOn = []string{}
		}
		info := pluginstate.Info{
			ID:          meta.ID,
			Name:        meta.Name,
			Version:     meta.Version,
			Description: meta.Description,
			DependsOn:   dependsOn,
			Builtin:     meta.Builtin,
			Enabled:     s.Enabled(meta.ID),
			LastError:   record.LastError,
		}
		if source, ok := s.sources[meta.ID]; ok {
			pending, pendingErr := s.engine.Pending(ctx, source)
			if pendingErr != nil {
				return nil, fmt.Errorf("count pending migrations for %q: %w", meta.ID, pendingErr)
			}
			info.PendingMigrations = pending
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// Enable runs the plugin's pending migrations, then flips it on.
//
// The state is re-checked after the advisory lock is taken, because another
// replica may have changed it in the meantime.
func (s *Service) Enable(ctx context.Context, pluginID string) error {
	meta, ok := s.byID[pluginID]
	if !ok {
		return pluginstate.ErrNotFound
	}
	if meta.Builtin {
		return pluginstate.ErrBuiltin
	}
	if err := s.reload(ctx); err != nil {
		return err
	}
	if s.Enabled(pluginID) {
		return nil
	}
	if missing := s.disabledDependencies(pluginID); len(missing) > 0 {
		return fmt.Errorf("%w: %s", pluginstate.ErrDependencyDisabled, strings.Join(missing, ", "))
	}

	release, err := s.repo.AcquireMigrationLock(ctx, toggleLockTimeout)
	if err != nil {
		return lockError(err)
	}
	defer release()

	if err := s.reload(ctx); err != nil {
		return err
	}
	if s.Enabled(pluginID) {
		return nil
	}
	if missing := s.disabledDependencies(pluginID); len(missing) > 0 {
		return fmt.Errorf("%w: %s", pluginstate.ErrDependencyDisabled, strings.Join(missing, ", "))
	}

	if source, hasMigrations := s.sources[pluginID]; hasMigrations {
		if err := s.engine.Run(ctx, source); err != nil {
			if setErr := s.repo.SetPluginError(ctx, pluginID, err.Error()); setErr != nil {
				logx.Errorf("record plugin %q error: %v", pluginID, setErr)
			}
			// enabled stays false: the flag is only set after every file
			// committed, so a half-applied plugin is never live.
			return fmt.Errorf("%w: %v", pluginstate.ErrMigrationFailed, err)
		}
	}

	// Before the flag flips: a button sync failure must leave the plugin off
	// rather than half enabled. The migration above may have just created the
	// page these buttons hang under.
	if err := s.syncPermissionMenus(ctx); err != nil {
		return err
	}

	updated, err := s.repo.SetPluginEnabled(ctx, pluginID, true)
	if err != nil {
		return err
	}
	if !updated {
		return pluginstate.ErrNotFound
	}
	if err := s.repo.SetPluginError(ctx, pluginID, ""); err != nil {
		logx.Errorf("clear plugin %q error: %v", pluginID, err)
	}
	return s.reload(ctx)
}

// Disable flips the flag off. It never runs DDL: tables and data stay exactly
// where they are, so re-enabling is instant and lossless.
func (s *Service) Disable(ctx context.Context, pluginID string) error {
	meta, ok := s.byID[pluginID]
	if !ok {
		return pluginstate.ErrNotFound
	}
	if meta.Builtin {
		return pluginstate.ErrBuiltin
	}

	release, err := s.repo.AcquireMigrationLock(ctx, toggleLockTimeout)
	if err != nil {
		return lockError(err)
	}
	defer release()

	if err := s.reload(ctx); err != nil {
		return err
	}
	if inUse := s.enabledDependents(pluginID); len(inUse) > 0 {
		return fmt.Errorf("%w: %s", pluginstate.ErrDependencyInUse, strings.Join(inUse, ", "))
	}

	updated, err := s.repo.SetPluginEnabled(ctx, pluginID, false)
	if err != nil {
		return err
	}
	if !updated {
		return pluginstate.ErrNotFound
	}
	return s.reload(ctx)
}

// Uninstall removes a disabled plugin completely.
//
// Unlike Disable, this is destructive: the plugin's declared tables, its menus
// (pages and the permission buttons hanging under them), its migration
// bookkeeping and its catalog row are deleted in one transaction. The plugin
// stays compiled in, so the next startup's catalog sync re-inserts it as
// disabled and enabling it again replays migration 001 from scratch.
//
// Requiring a disabled plugin keeps the two operations separate: "stop serving
// traffic" and "throw the data away" must never be one click.
func (s *Service) Uninstall(ctx context.Context, pluginID string) error {
	meta, ok := s.byID[pluginID]
	if !ok {
		return pluginstate.ErrNotFound
	}
	if meta.Builtin {
		return pluginstate.ErrBuiltin
	}

	release, err := s.repo.AcquireMigrationLock(ctx, toggleLockTimeout)
	if err != nil {
		return lockError(err)
	}
	defer release()

	// Re-read under the lock: another replica may have enabled it meanwhile.
	if err := s.reload(ctx); err != nil {
		return err
	}
	if s.Enabled(pluginID) {
		return pluginstate.ErrPluginEnabled
	}
	if inUse := s.enabledDependents(pluginID); len(inUse) > 0 {
		return fmt.Errorf("%w: %s", pluginstate.ErrDependencyInUse, strings.Join(inUse, ", "))
	}

	if err := s.repo.UninstallPlugin(ctx, pluginID, meta.DropTables); err != nil {
		return err
	}
	return s.reload(ctx)
}

// DefaultWatchInterval is how often a replica re-reads the plugin state that
// another replica may have changed.
const DefaultWatchInterval = 5 * time.Second

// Watch keeps this replica's snapshot aligned with the database.
//
// Enable/Disable already reload the snapshot of the replica that served the
// request, but every other replica would keep its old view until it restarted.
// Watch closes that gap: it re-reads sys_plugin on a timer, so a toggle becomes
// visible everywhere within one interval.
//
// It polls instead of subscribing to Redis on purpose. go-zero's redis client
// exposes no subscriber, and a poll of a table with one row per compiled-in
// plugin is cheaper than a second connection plus a pub/sub protocol that has
// to handle reconnects and dropped messages. The cost of the remaining window
// is bounded: at worst another replica serves a disabled plugin's routes for
// one interval.
//
// It blocks until ctx is cancelled, so callers run it in a goroutine.
func (s *Service) Watch(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = DefaultWatchInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.reload(ctx); err != nil {
				// A transient database error must not kill the watcher; the
				// next tick retries and the snapshot stays as it was.
				logx.Errorf("watch plugin state: %v", err)
			}
		}
	}
}

// permissionRoutes projects the route table into the button catalog. AnyOf
// codes are included: they are granted through the same mechanism.
func permissionRoutes(bounds []kernel.Bound, builtin map[string]bool) []menuperm.Route {
	routes := make([]menuperm.Route, 0, len(bounds))
	for _, bound := range bounds {
		codes := make([]string, 0, len(bound.Route.AnyOf)+1)
		if bound.Route.Permission != "" {
			codes = append(codes, bound.Route.Permission)
		}
		codes = append(codes, bound.Route.AnyOf...)
		routes = append(routes, menuperm.Route{
			PluginID: bound.PluginID,
			Codes:    codes,
			MenuCode: bound.Route.MenuCode,
			MenuName: bound.Route.MenuName,
			Method:   bound.Route.Method,
			Path:     bound.Route.Path,
			Builtin:  builtin[bound.PluginID],
		})
	}
	return routes
}

// syncPermissionMenus makes the button nodes match the route table.
//
// A core route that resolves to no page is a programming error and fails
// startup. A plugin route may legitimately have no page yet (its migration has
// not run), so it is skipped; enabling the plugin re-runs this.
func (s *Service) syncPermissionMenus(ctx context.Context) error {
	index, err := s.repo.MenuCodeIndex(ctx)
	if err != nil {
		return fmt.Errorf("load menu index: %w", err)
	}
	buttons, unresolved := menuperm.Build(s.routes, index)
	for _, item := range unresolved {
		if item.Builtin {
			return fmt.Errorf("permission %q of built-in plugin %q has no owning menu",
				item.Code, item.PluginID)
		}
		logx.Infof("permission %q skipped: plugin %q has no menu yet", item.Code, item.PluginID)
	}
	return s.repo.SyncPermissionMenus(ctx, buttons)
}

// reload rebuilds the in-memory snapshot from the database.
func (s *Service) reload(ctx context.Context) error {
	records, err := s.repo.ListPlugins(ctx)
	if err != nil {
		return fmt.Errorf("reload plugin state: %w", err)
	}

	next := &snapshot{
		enabled: make(map[string]bool, len(records)),
		builtin: make(map[string]bool, len(s.metas)),
	}
	for _, record := range records {
		next.enabled[record.ID] = record.Enabled
	}
	// The catalog is the source of truth for which plugins exist and which are
	// builtin; builtins are unconditionally live.
	for _, meta := range s.metas {
		next.builtin[meta.ID] = meta.Builtin
		if meta.Builtin {
			next.enabled[meta.ID] = true
		}
	}
	s.state.Store(next)
	return nil
}

func (s *Service) catalogEntries() []store.PluginCatalogEntry {
	entries := make([]store.PluginCatalogEntry, 0, len(s.metas))
	for _, meta := range s.metas {
		entries = append(entries, store.PluginCatalogEntry{
			ID:          meta.ID,
			Name:        meta.Name,
			Version:     meta.Version,
			Description: meta.Description,
			Builtin:     meta.Builtin,
		})
	}
	return entries
}

// disabledDependencies lists the declared dependencies that are not live.
func (s *Service) disabledDependencies(pluginID string) []string {
	var missing []string
	for _, dep := range s.byID[pluginID].DependsOn {
		if !s.Enabled(dep) {
			missing = append(missing, dep)
		}
	}
	return missing
}

// enabledDependents lists the live plugins that would break if pluginID were
// switched off.
func (s *Service) enabledDependents(pluginID string) []string {
	var users []string
	for _, meta := range s.metas {
		if meta.ID == pluginID || !s.Enabled(meta.ID) {
			continue
		}
		for _, dep := range meta.DependsOn {
			if dep == pluginID {
				users = append(users, meta.ID)
				break
			}
		}
	}
	return users
}

func lockError(err error) error {
	if errors.Is(err, store.ErrMigrationBusy) {
		return pluginstate.ErrMigrationBusy
	}
	return err
}

func checksum(script string) string {
	sum := sha256.Sum256([]byte(script))
	return hex.EncodeToString(sum[:])
}
