package plugincenter

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"dragz-admin/backend/internal/kernel"
	"dragz-admin/backend/internal/menuperm"
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/internal/pluginstate"
	"dragz-admin/backend/internal/store"
)

// fakeRepo mimics the SQL semantics of PluginRepository closely enough to test
// the service: a new row starts at enabled = is_builtin, an existing row keeps
// the administrator's choice, and builtins can never be toggled.
type fakeRepo struct {
	records      map[string]store.PluginRecord
	applied      map[string]map[string]string
	failVersions map[string]bool
	lockHeld     bool
	applyCalls   int
	// menuCodes is the sys_menu.code -> id index the button sync resolves
	// parents against; syncedButtons records what the sync wrote.
	menuCodes     map[string]uint64
	syncedButtons []menuperm.Button
	syncCalls     int
	// uninstallCalls and droppedTables record the last Uninstall request.
	uninstallCalls int
	droppedTables  []string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		records:      map[string]store.PluginRecord{},
		applied:      map[string]map[string]string{},
		failVersions: map[string]bool{},
		menuCodes:    map[string]uint64{},
	}
}

func (f *fakeRepo) MenuCodeIndex(context.Context) (map[string]uint64, error) {
	return f.menuCodes, nil
}

func (f *fakeRepo) SyncPermissionMenus(_ context.Context, buttons []menuperm.Button) error {
	f.syncCalls++
	f.syncedButtons = buttons
	return nil
}

func (f *fakeRepo) Bootstrap(context.Context) error { return nil }
func (f *fakeRepo) Seed(context.Context) error      { return nil }

func (f *fakeRepo) SyncCatalog(_ context.Context, entries []store.PluginCatalogEntry) error {
	for _, entry := range entries {
		record, exists := f.records[entry.ID]
		if !exists {
			record = store.PluginRecord{ID: entry.ID, Enabled: entry.Builtin}
		}
		record.Name = entry.Name
		record.Version = entry.Version
		record.Description = entry.Description
		record.Builtin = entry.Builtin
		if entry.Builtin {
			record.Enabled = true
		}
		f.records[entry.ID] = record
	}
	return nil
}

func (f *fakeRepo) ListPlugins(context.Context) ([]store.PluginRecord, error) {
	out := make([]store.PluginRecord, 0, len(f.records))
	for _, record := range f.records {
		out = append(out, record)
	}
	return out, nil
}

func (f *fakeRepo) SetPluginEnabled(_ context.Context, pluginID string, enabled bool) (bool, error) {
	record, ok := f.records[pluginID]
	if !ok || record.Builtin {
		return false, nil
	}
	record.Enabled = enabled
	f.records[pluginID] = record
	return true, nil
}

func (f *fakeRepo) SetPluginError(_ context.Context, pluginID, message string) error {
	record := f.records[pluginID]
	record.LastError = message
	f.records[pluginID] = record
	return nil
}

func (f *fakeRepo) AppliedMigrations(_ context.Context, pluginID string) (map[string]string, error) {
	out := map[string]string{}
	for version, sum := range f.applied[pluginID] {
		out[version] = sum
	}
	return out, nil
}

func (f *fakeRepo) ApplyMigration(_ context.Context, pluginID, version, _, sum, _ string) error {
	f.applyCalls++
	if f.failVersions[pluginID+"/"+version] {
		return errors.New("synthetic migration failure")
	}
	if f.applied[pluginID] == nil {
		f.applied[pluginID] = map[string]string{}
	}
	f.applied[pluginID][version] = sum
	return nil
}

func (f *fakeRepo) AcquireMigrationLock(context.Context, time.Duration) (func(), error) {
	if f.lockHeld {
		return nil, store.ErrMigrationBusy
	}
	f.lockHeld = true
	return func() { f.lockHeld = false }, nil
}

// UninstallPlugin records what it was asked to drop, then mirrors the real
// repository's effect on the in-memory catalog.
func (f *fakeRepo) UninstallPlugin(_ context.Context, pluginID string, dropTables []string) error {
	f.uninstallCalls++
	f.droppedTables = append([]string(nil), dropTables...)
	delete(f.records, pluginID)
	delete(f.applied, pluginID)
	return nil
}

func metas() []plugin.Meta {
	return []plugin.Meta{
		{ID: "system", Name: "系统管理", Version: "1.0.0", Builtin: true},
		{ID: "demo", Name: "示例插件", Version: "0.1.0", DependsOn: []string{"system"},
			DropTables: []string{"demo"}},
		{ID: "extra", Name: "依赖 demo 的插件", Version: "0.1.0", DependsOn: []string{"demo"}},
	}
}

func sources() []plugin.MigrationSource {
	return []plugin.MigrationSource{{
		PluginID: "demo",
		Migrations: []plugin.Migration{
			{Version: "001", Name: "init", SQL: "CREATE TABLE demo (id BIGINT);"},
			{Version: "002", Name: "seed", SQL: "INSERT INTO demo VALUES (1);"},
		},
	}}
}

func newService(t *testing.T) (*Service, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	service, err := New(repo, metas(), sources(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return service, repo
}

func TestNewRejectsBadMigrationVersions(t *testing.T) {
	repo := newFakeRepo()
	_, err := New(repo, metas(), []plugin.MigrationSource{{
		PluginID:   "demo",
		Migrations: []plugin.Migration{{Version: "1", Name: "bad", SQL: "SELECT 1"}},
	}}, nil)
	if err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("err = %v, want a version-format error", err)
	}
}

func TestNewRejectsDuplicateVersions(t *testing.T) {
	repo := newFakeRepo()
	_, err := New(repo, metas(), []plugin.MigrationSource{{
		PluginID: "demo",
		Migrations: []plugin.Migration{
			{Version: "001", Name: "a", SQL: "SELECT 1"},
			{Version: "001", Name: "b", SQL: "SELECT 2"},
		},
	}}, nil)
	if err == nil {
		t.Fatal("New: want a duplicate-version error")
	}
}

// Startup must leave builtins on and everything else off (ship dark).
func TestRunStartupShipsNewPluginsDisabled(t *testing.T) {
	service, _ := newService(t)

	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if !service.Enabled("system") {
		t.Fatal("system must be enabled")
	}
	if service.Enabled("demo") || service.Enabled("extra") {
		t.Fatal("non-builtin plugins must start disabled")
	}
	if !service.Builtin("system") || service.Builtin("demo") {
		t.Fatal("builtin flags are wrong")
	}
}

func TestEnableRunsMigrationsThenFlipsFlag(t *testing.T) {
	service, repo := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}

	if err := service.Enable(context.Background(), "demo"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if !service.Enabled("demo") {
		t.Fatal("demo must be enabled")
	}
	if len(repo.applied["demo"]) != 2 {
		t.Fatalf("applied = %v, want both files", repo.applied["demo"])
	}

	// Re-enabling must be a no-op: no duplicate migration rows.
	before := repo.applyCalls
	if err := service.Enable(context.Background(), "demo"); err != nil {
		t.Fatalf("Enable again: %v", err)
	}
	if repo.applyCalls != before {
		t.Fatalf("applyCalls = %d, want %d (nothing to do)", repo.applyCalls, before)
	}
}

func TestEnableRejectsDisabledDependency(t *testing.T) {
	service, _ := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}

	err := service.Enable(context.Background(), "extra")
	if !errors.Is(err, pluginstate.ErrDependencyDisabled) {
		t.Fatalf("err = %v, want ErrDependencyDisabled", err)
	}
}

func TestEnableKeepsPluginOffWhenMigrationFails(t *testing.T) {
	service, repo := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	repo.failVersions["demo/002"] = true

	err := service.Enable(context.Background(), "demo")
	if !errors.Is(err, pluginstate.ErrMigrationFailed) {
		t.Fatalf("err = %v, want ErrMigrationFailed", err)
	}
	if service.Enabled("demo") {
		t.Fatal("demo must stay disabled after a failed migration")
	}
	if repo.records["demo"].LastError == "" {
		t.Fatal("last_error should explain the failure")
	}
	if len(repo.applied["demo"]) != 1 {
		t.Fatalf("applied = %v, want only 001 (resumable)", repo.applied["demo"])
	}
}

func TestDisableRejectsBuiltinAndInUse(t *testing.T) {
	service, _ := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if err := service.Enable(context.Background(), "demo"); err != nil {
		t.Fatalf("Enable demo: %v", err)
	}
	if err := service.Enable(context.Background(), "extra"); err != nil {
		t.Fatalf("Enable extra: %v", err)
	}

	if err := service.Disable(context.Background(), "system"); !errors.Is(err, pluginstate.ErrBuiltin) {
		t.Fatalf("Disable(system) = %v, want ErrBuiltin", err)
	}
	if err := service.Disable(context.Background(), "demo"); !errors.Is(err, pluginstate.ErrDependencyInUse) {
		t.Fatalf("Disable(demo) = %v, want ErrDependencyInUse", err)
	}

	if err := service.Disable(context.Background(), "extra"); err != nil {
		t.Fatalf("Disable(extra): %v", err)
	}
	if err := service.Disable(context.Background(), "demo"); err != nil {
		t.Fatalf("Disable(demo): %v", err)
	}
	if service.Enabled("demo") {
		t.Fatal("demo must be disabled")
	}
	// Disabling keeps the migration bookkeeping, so re-enabling is instant.
	if len(repo0(service).applied["demo"]) != 2 {
		t.Fatal("migration rows must survive a disable")
	}
}

func TestDisableUnknownPlugin(t *testing.T) {
	service, _ := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if err := service.Disable(context.Background(), "nope"); !errors.Is(err, pluginstate.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// Uninstall is destructive, so every guard must hold before any DDL runs.
func TestUninstallGuards(t *testing.T) {
	service, repo := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if err := service.Enable(context.Background(), "demo"); err != nil {
		t.Fatalf("Enable demo: %v", err)
	}
	if err := service.Enable(context.Background(), "extra"); err != nil {
		t.Fatalf("Enable extra: %v", err)
	}

	if err := service.Uninstall(context.Background(), "nope"); !errors.Is(err, pluginstate.ErrNotFound) {
		t.Fatalf("Uninstall(nope) = %v, want ErrNotFound", err)
	}
	if err := service.Uninstall(context.Background(), "system"); !errors.Is(err, pluginstate.ErrBuiltin) {
		t.Fatalf("Uninstall(system) = %v, want ErrBuiltin", err)
	}
	// An enabled plugin must be stopped before its data is thrown away.
	if err := service.Uninstall(context.Background(), "demo"); !errors.Is(err, pluginstate.ErrPluginEnabled) {
		t.Fatalf("Uninstall(demo, enabled) = %v, want ErrPluginEnabled", err)
	}
	if repo.uninstallCalls != 0 {
		t.Fatalf("UninstallPlugin called %d times, want 0 while guards reject", repo.uninstallCalls)
	}

	if err := service.Disable(context.Background(), "extra"); err != nil {
		t.Fatalf("Disable extra: %v", err)
	}
	// Now nothing depends on demo being live, so the guard that blocked the
	// uninstall is gone; the remaining check (enabled) still rejects.
	if err := service.Uninstall(context.Background(), "demo"); !errors.Is(err, pluginstate.ErrPluginEnabled) {
		t.Fatalf("Uninstall(demo, enabled) = %v, want ErrPluginEnabled", err)
	}
	if repo.uninstallCalls != 0 {
		t.Fatalf("UninstallPlugin called %d times, want 0 while a guard rejects", repo.uninstallCalls)
	}
}

func TestUninstallDropsDeclaredTablesAndResetsPlugin(t *testing.T) {
	service, repo := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if err := service.Enable(context.Background(), "demo"); err != nil {
		t.Fatalf("Enable demo: %v", err)
	}
	if err := service.Disable(context.Background(), "demo"); err != nil {
		t.Fatalf("Disable demo: %v", err)
	}

	if err := service.Uninstall(context.Background(), "demo"); err != nil {
		t.Fatalf("Uninstall(demo): %v", err)
	}
	if repo.uninstallCalls != 1 {
		t.Fatalf("UninstallPlugin called %d times, want 1", repo.uninstallCalls)
	}
	if len(repo.droppedTables) != 1 || repo.droppedTables[0] != "demo" {
		t.Fatalf("dropped tables = %v, want [demo]", repo.droppedTables)
	}
	// The catalog row and the migration bookkeeping are gone, so the plugin is
	// back to "never installed": enabling it replays 001 from scratch.
	if _, exists := repo.records["demo"]; exists {
		t.Fatal("catalog row must be removed")
	}
	if len(repo.applied["demo"]) != 0 {
		t.Fatalf("migration rows = %v, want none", repo.applied["demo"])
	}
	if service.Enabled("demo") {
		t.Fatal("demo must not be enabled after uninstall")
	}
}

// A non-builtin plugin whose startup migration fails is disabled, but the
// process still boots.
func TestRunStartupAutoDisablesOnMigrationFailure(t *testing.T) {
	repo := newFakeRepo()
	service, err := New(repo, metas(), sources(), nil)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Pretend an earlier run enabled demo, and now its migration breaks.
	repo.records["demo"] = store.PluginRecord{ID: "demo", Enabled: true}
	repo.failVersions["demo/001"] = true

	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup must not fail for a non-builtin plugin: %v", err)
	}
	if service.Enabled("demo") {
		t.Fatal("demo must be auto-disabled")
	}
	if repo.records["demo"].LastError == "" {
		t.Fatal("last_error should explain the auto-disable")
	}
}

func TestListReportsPendingMigrations(t *testing.T) {
	service, _ := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}

	infos, err := service.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	byID := map[string]pluginstate.Info{}
	for _, info := range infos {
		byID[info.ID] = info
	}
	if got := byID["demo"].PendingMigrations; got != 2 {
		t.Fatalf("demo pending = %d, want 2", got)
	}
	if got := byID["system"].PendingMigrations; got != 0 {
		t.Fatalf("system pending = %d, want 0", got)
	}
	if byID["demo"].DependsOn == nil {
		t.Fatal("DependsOn must never be nil in the JSON payload")
	}

	// Once applied, nothing is pending.
	if err := service.Enable(context.Background(), "demo"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	infos, err = service.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, info := range infos {
		if info.ID == "demo" && info.PendingMigrations != 0 {
			t.Fatalf("demo pending after enable = %d, want 0", info.PendingMigrations)
		}
	}
}

func TestEnableReportsBusyLock(t *testing.T) {
	service, repo := newService(t)
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	repo.lockHeld = true

	err := service.Enable(context.Background(), "demo")
	if !errors.Is(err, pluginstate.ErrMigrationBusy) {
		t.Fatalf("err = %v, want ErrMigrationBusy", err)
	}
}

// repo0 is a test helper that reaches the fake repository behind a service.
func repo0(s *Service) *fakeRepo {
	return s.repo.(*fakeRepo)
}

// routeBound builds a kernel.Bound the way the registry would.
func routeBound(pluginID, permission, method, path string) kernel.Bound {
	return kernel.Bound{
		PluginID: pluginID,
		Route:    plugin.Route{Method: method, Path: path, Permission: permission},
	}
}

func TestRunStartupSyncsPermissionButtons(t *testing.T) {
	repo := newFakeRepo()
	repo.menuCodes = map[string]uint64{"system_user": 11}
	service, err := New(repo, metas(), sources(), []kernel.Bound{
		routeBound("system", "system:user:query", "GET", "/api/system/users"),
		routeBound("system", "system:user:create", "POST", "/api/system/users"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if len(repo.syncedButtons) != 2 {
		t.Fatalf("expected 2 buttons, got %d", len(repo.syncedButtons))
	}
	if repo.syncedButtons[0].ParentID != 11 || repo.syncedButtons[0].Code != "system:user:query" {
		t.Fatalf("unexpected button: %+v", repo.syncedButtons[0])
	}
	if repo.syncedButtons[0].Name != "查询用户" {
		t.Fatalf("name = %q, want 查询用户", repo.syncedButtons[0].Name)
	}
}

// A core route without an owning menu is a programming error: the button would
// be ungrantable and invisible, so the process must refuse to start.
func TestRunStartupRejectsBuiltinPermissionWithoutMenu(t *testing.T) {
	repo := newFakeRepo()
	service, err := New(repo, metas(), sources(), []kernel.Bound{
		routeBound("system", "system:missing:query", "GET", "/api/system/missing"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	err = service.RunStartup(context.Background())
	if err == nil || !strings.Contains(err.Error(), "system:missing:query") {
		t.Fatalf("err = %v, want a missing-menu error naming the code", err)
	}
}

// A plugin whose page does not exist yet is normal before its first enable.
func TestRunStartupSkipsPluginPermissionWithoutMenu(t *testing.T) {
	repo := newFakeRepo()
	service, err := New(repo, metas(), sources(), []kernel.Bound{
		routeBound("demo", "demo:ping", "GET", "/api/demo/ping"),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := service.RunStartup(context.Background()); err != nil {
		t.Fatalf("RunStartup: %v", err)
	}
	if len(repo.syncedButtons) != 0 {
		t.Fatalf("expected no buttons, got %d", len(repo.syncedButtons))
	}
}
