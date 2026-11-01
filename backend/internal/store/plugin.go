package store

import (
	"context"
	"fmt"
	"regexp"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// migrationLockKey is the pg_advisory_lock key that serialises every schema
// mutation (bootstrap, seed, catalog sync, startup migrations, enable and
// disable) across processes. The hex spells "dragzplg".
const migrationLockKey int64 = 0x647261677A706C67

// maxLastErrorLen keeps a runaway SQL error from filling the column.
const maxLastErrorLen = 1000

// pluginIDPattern mirrors plugin.ValidID. It is duplicated instead of imported
// because store must not depend on internal/plugin (store → plugin → app →
// store). Only UninstallPlugin uses it, and only to refuse an identifier it is
// about to put in a DDL statement.
var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]{2,15}$`)

// tableNamePattern is the only shape UninstallPlugin will interpolate into
// DROP TABLE. It excludes schema qualification, quoting and anything else that
// could turn a declared table name into a different statement.
var tableNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// PluginCatalogEntry is the neutral projection of a plugin declaration that
// the store accepts. It exists so this package does not import
// internal/plugin, which would create a cycle
// (store → plugin → app → store).
type PluginCatalogEntry struct {
	ID          string
	Name        string
	Version     string
	Description string
	Builtin     bool
}

// PluginRecord is the persisted state of one plugin.
type PluginRecord struct {
	ID          string
	Name        string
	Version     string
	Description string
	Builtin     bool
	Enabled     bool
	LastError   string
	EnabledAt   time.Time
	UpdatedAt   time.Time
}

// PluginRepository owns sys_plugin, sys_plugin_migration and the idempotent
// bootstrap DDL that creates them on both fresh and existing databases.
type PluginRepository struct {
	db      *DB
	queries *sqlcstore.Queries
}

// NewPluginRepository binds the generated queries to a database handle.
func NewPluginRepository(db *DB) *PluginRepository {
	return &PluginRepository{db: db, queries: sqlcstore.New(db)}
}

// bootstrapStatements must stay idempotent: they run on every startup, against
// a brand-new database and against one created months ago.
var bootstrapStatements = []string{
	`CREATE TABLE IF NOT EXISTS sys_plugin (
  plugin_id VARCHAR(16) PRIMARY KEY,
  name VARCHAR(128) NOT NULL DEFAULT '',
  version VARCHAR(32) NOT NULL DEFAULT '',
  description VARCHAR(500) NOT NULL DEFAULT '',
  is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  last_error TEXT NOT NULL DEFAULT '',
  enabled_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
	`CREATE TABLE IF NOT EXISTS sys_plugin_migration (
  plugin_id VARCHAR(16) NOT NULL,
  version VARCHAR(32) NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  checksum VARCHAR(64) NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (plugin_id, version)
)`,
	// '' = core menu (always visible). The default backfills existing rows
	// without a table rewrite on PG 11+.
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS plugin_id VARCHAR(16) NOT NULL DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_menu_plugin_id ON sys_menu (plugin_id)`,
	// Permission buttons carry the endpoint they guard. Owned by the startup
	// sync (see SyncPermissionMenus), never by the admin UI.
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS method VARCHAR(16) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS path VARCHAR(255) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS is_sensitive SMALLINT NOT NULL DEFAULT 0`,
	// 1 = the row belongs to the route-table sync. Existing rows start at 0 and
	// are flipped by the first sync, which is harmless: the sync only refuses to
	// retire rows it owns, and everything it owns is re-declared on every boot.
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS managed SMALLINT NOT NULL DEFAULT 0`,
	// Dead since permission codes moved to sys_menu.code: the column was never
	// populated by the seed and never read by any query.
	`ALTER TABLE sys_menu DROP COLUMN IF EXISTS permission`,
	// Business keys are unique among LIVE rows only (see schema/tables.sql).
	// A database created before this change still carries a table constraint
	// that keeps a deleted row's code occupied, which made "delete a record,
	// then create it again with the same code" fail with a duplicate-key
	// error. Dropping the constraint and installing the partial index fixes
	// that in place: the tombstones survive, the codes they hold do not block
	// new records. Both halves are no-ops on a database built from the current
	// migrations, so this block can stay forever.
	`ALTER TABLE sys_department DROP CONSTRAINT IF EXISTS uk_department_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_department_code ON sys_department (code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_position DROP CONSTRAINT IF EXISTS uk_position_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_position_code ON sys_position (code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_user DROP CONSTRAINT IF EXISTS uk_user_username`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_user_username ON sys_user (username) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_role DROP CONSTRAINT IF EXISTS uk_role_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_role_code ON sys_role (code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_menu DROP CONSTRAINT IF EXISTS uk_menu_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_menu_code ON sys_menu (code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_dict_type DROP CONSTRAINT IF EXISTS uk_dict_type_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_type_code ON sys_dict_type (code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_dict_item DROP CONSTRAINT IF EXISTS uk_dict_item_value`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_item_value ON sys_dict_item (dict_type_id, value) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_scheduled_job DROP CONSTRAINT IF EXISTS uk_scheduled_job_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_scheduled_job_code ON sys_scheduled_job (code) WHERE deleted_at IS NULL`,
	// The translation page reads every preset text through these two views.
	// They live in migrations/001_system_init.sql, which the Postgres image only
	// runs against an empty data directory — a database initialised before the
	// views were added never gets them, and the page answers 500 with
	// `relation "v_preset_entity" does not exist`. Recreating them on every boot
	// is what makes the page work on an already-initialised database.
	//
	// Keep these definitions identical to schema/tables.sql: sqlc type-checks the
	// queries against that file, so a drift here only shows up at runtime.
	`CREATE OR REPLACE VIEW v_preset_entity AS
  SELECT 'menu.' || code AS entity_key, name AS source, updated_at FROM sys_menu
    WHERE deleted_at IS NULL AND type IN ('page', 'directory')
  UNION ALL
  SELECT 'perm.' || code, name, updated_at FROM sys_menu
    WHERE deleted_at IS NULL AND type = 'button'
  UNION ALL
  SELECT 'role.' || code, name, updated_at FROM sys_role WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'role.' || code || '.remark', remark, updated_at FROM sys_role
    WHERE deleted_at IS NULL AND remark <> ''
  UNION ALL
  SELECT 'dept.' || code, name, updated_at FROM sys_department WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'post.' || code, name, updated_at FROM sys_position WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'post.' || code || '.remark', remark, updated_at FROM sys_position
    WHERE deleted_at IS NULL AND remark <> ''
  UNION ALL
  SELECT 'job.' || code, name, updated_at FROM sys_scheduled_job WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'storage.' || storage_type, name, updated_at FROM sys_storage_config
    WHERE deleted_at IS NULL`,
	`CREATE OR REPLACE VIEW v_preset_source_row AS
  SELECT 'menu'::text AS namespace, id, code AS code, name AS source,
         ''::text AS remark
  FROM sys_menu WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'role', id, code, name, remark FROM sys_role WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'dept', id, code, name, '' FROM sys_department WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'post', id, code, name, remark FROM sys_position WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'job', id, code, name, '' FROM sys_scheduled_job WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'storage', id, storage_type, name, '' FROM sys_storage_config
    WHERE deleted_at IS NULL`,
}

// retiredModuleStatements delete rows belonging to product modules that were
// removed from the code base.
//
// They exist because there is no migration runner: the Postgres image only
// executes migrations/*.sql against an empty data directory, so deleting a seed
// file does nothing for databases that are already initialised. Running these
// on every boot is safe — once the rows are gone the statements match nothing.
//
// Every retired module must be listed here at removal time, and the block can be
// dropped once no deployment can still hold the old rows.
var retiredModuleStatements = []string{
	// AI 创作 (removed 2026-10-29). It was a frontend-only module: menu rows
	// only, no API endpoints and no permission codes.
	`DELETE FROM sys_role_menu WHERE menu_id IN (
  SELECT id FROM sys_menu
  WHERE code IN ('ai_creation', 'ai_image', 'ai_video', 'ai_works', 'ai_ops')
)`,
	`DELETE FROM sys_menu
  WHERE code IN ('ai_creation', 'ai_image', 'ai_video', 'ai_works', 'ai_ops')`,
	// 接口管理 (removed 2026-10-29). Permission codes now live as button nodes
	// under the page they belong to, so the standalone page and its own
	// permission codes are gone.
	`DELETE FROM sys_role_menu WHERE menu_id IN (
  SELECT id FROM sys_menu
  WHERE code = 'system_api_permission' OR code LIKE 'system:api-permission:%'
)`,
	`DELETE FROM sys_menu
  WHERE code = 'system_api_permission' OR code LIKE 'system:api-permission:%'`,
}

// seedStatements are idempotent DML. They run after bootstrap, under the same
// advisory lock as everything else.
var seedStatements = []string{
	// migrations/001_system_init.sql seeds rows with EXPLICIT ids, which leaves
	// the owning sequence at its initial value. The first row created through
	// the UI then asks for id 1 again and dies with
	// `duplicate key value violates unique constraint "sys_x_pkey"` — the
	// reason "delete a record, then create another one" appeared to fail on
	// almost every table.
	//
	// This used to be a single setval() for sys_menu only, so every other table
	// stayed broken until someone happened to trigger (and absorb) its
	// collision. Walking every serial id column instead means a table added
	// later cannot be forgotten. GREATEST keeps each sequence monotonic, so the
	// whole block is a no-op once the values line up and is safe on every boot.
	`DO $$
DECLARE
  target RECORD;
BEGIN
  FOR target IN
    SELECT c.relname AS tbl, a.attname AS col, s.relname AS seq
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_depend d ON d.refobjid = c.oid AND d.deptype = 'a'
    JOIN pg_class s ON s.oid = d.objid AND s.relkind = 'S'
    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = d.refobjsubid
    WHERE n.nspname = current_schema() AND c.relkind = 'r'
  LOOP
    EXECUTE format(
      'SELECT setval(%L, GREATEST((SELECT COALESCE(MAX(%I), 0) FROM %I), (SELECT last_value FROM %I)))',
      target.seq, target.col, target.tbl, target.seq
    );
  END LOOP;
END
$$`,
}

// Bootstrap creates the plugin tables/columns if they are missing.
func (r *PluginRepository) Bootstrap(ctx context.Context) error {
	for _, statement := range bootstrapStatements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap plugin schema: %w", err)
		}
	}
	return r.migrateLegacyAPIPermissions(ctx)
}

// legacyAPIMigrationStatements convert the old dedicated permission tables into
// menu buttons and then drop them.
//
// The grant rows are the only data that cannot be re-derived, so they are copied
// first: sys_role_api_permission becomes sys_role_menu pointing at the matching
// button. parent_id starts at 0 and is corrected by SyncPermissionMenus, which
// runs after every plugin migration has created its pages.
var legacyAPIMigrationStatements = []string{
	`INSERT INTO sys_menu
  (parent_id, type, name, code, method, path, is_sensitive, sort, status, visible, plugin_id)
SELECT 0, 'button', p.name, p.code, p.method, p.path, p.is_sensitive, 100, 1, 0, ''
FROM sys_api_permission p
ON CONFLICT DO NOTHING`,
	`INSERT INTO sys_role_menu (role_id, menu_id)
SELECT rp.role_id, m.id
FROM sys_role_api_permission rp
INNER JOIN sys_api_permission p ON p.id = rp.permission_id
INNER JOIN sys_menu m ON m.code = p.code AND m.type = 'button'
ON CONFLICT DO NOTHING`,
	// Dependents first: the join table, then the grant table, then the catalog.
	`DROP TABLE IF EXISTS sys_menu_api_permission`,
	`DROP TABLE IF EXISTS sys_role_api_permission`,
	`DROP TABLE IF EXISTS sys_api_permission`,
}

// migrateLegacyAPIPermissions is a one-shot upgrade path from the separate
// "API permission" model to permission buttons.
//
// The guard is the presence of the legacy table, so the block is a no-op on
// every boot after the first. PostgreSQL runs DDL transactionally: a crash
// rolls back the backfill and the drops together, never leaving a database with
// the tables gone and the grants lost.
//
// The backfill copies every legacy code, including codes of modules that have
// since been deleted (接口管理 itself). Seed runs right after Bootstrap and its
// retiredModuleStatements delete exactly those rows again, so the net result is
// "grants for permissions that still exist survive, the rest disappear".
func (r *PluginRepository) migrateLegacyAPIPermissions(ctx context.Context) error {
	var legacy bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT to_regclass('public.sys_api_permission') IS NOT NULL`).Scan(&legacy); err != nil {
		return fmt.Errorf("check legacy permission table: %w", err)
	}
	if !legacy {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range legacyAPIMigrationStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate legacy api permissions: %w", err)
		}
	}
	return tx.Commit()
}

// Seed installs the plugin-center permission codes and menu row, then repairs
// the menu sequence. Every statement is idempotent.
//
// Retired-module cleanup runs first so a removed module's menu rows cannot
// outlive the code that rendered them.
func (r *PluginRepository) Seed(ctx context.Context) error {
	for _, statement := range retiredModuleStatements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("retire removed module: %w", err)
		}
	}
	if err := r.queries.SeedPluginMenu(ctx); err != nil {
		return fmt.Errorf("seed plugin menu: %w", err)
	}
	if err := r.queries.SeedPluginRoleMenu(ctx); err != nil {
		return fmt.Errorf("grant plugin menu to role 1: %w", err)
	}
	for _, statement := range seedStatements {
		if _, err := r.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("repair id sequences: %w", err)
		}
	}
	return nil
}

// SyncCatalog upserts one row per compiled-in plugin.
//
// A newly inserted non-builtin plugin starts disabled (ship dark): its
// migrations only run when an administrator enables it. Builtins are forced
// enabled so the platform can never be switched off. An existing row keeps the
// administrator's choice.
func (r *PluginRepository) SyncCatalog(ctx context.Context, entries []PluginCatalogEntry) error {
	for _, entry := range entries {
		if err := r.queries.UpsertPlugin(ctx, sqlcstore.UpsertPluginParams{
			PluginID:    entry.ID,
			Name:        entry.Name,
			Version:     entry.Version,
			Description: entry.Description,
			IsBuiltin:   entry.Builtin,
		}); err != nil {
			return fmt.Errorf("sync plugin %q: %w", entry.ID, err)
		}
	}
	return nil
}

// ListPlugins returns every persisted plugin row ordered by ID.
func (r *PluginRepository) ListPlugins(ctx context.Context) ([]PluginRecord, error) {
	rows, err := r.queries.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]PluginRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, PluginRecord{
			ID:          row.PluginID,
			Name:        row.Name,
			Version:     row.Version,
			Description: row.Description,
			Builtin:     row.IsBuiltin,
			Enabled:     row.Enabled,
			LastError:   row.LastError,
			EnabledAt:   row.EnabledAt.Time,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return records, nil
}

// SetPluginEnabled flips the flag. It reports false when no row was updated,
// which happens for an unknown plugin or for a builtin (the SQL excludes
// builtins as defense in depth).
func (r *PluginRepository) SetPluginEnabled(ctx context.Context, pluginID string, enabled bool) (bool, error) {
	var (
		affected int64
		err      error
	)
	if enabled {
		affected, err = r.queries.EnablePlugin(ctx, pluginID)
	} else {
		affected, err = r.queries.DisablePlugin(ctx, pluginID)
	}
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// UninstallPlugin removes a plugin completely.
//
// Everything runs in one transaction, so a failure leaves the plugin exactly as
// it was. Order matters only for foreign keys: grants before menus, and the
// tables before the catalog row that names them.
//
// Table names are interpolated (SQL cannot bind identifiers), so they are
// validated against a strict identifier pattern first. A plugin that declares
// no tables is still uninstallable: it just has menus and migrations.
func (r *PluginRepository) UninstallPlugin(ctx context.Context, pluginID string, dropTables []string) error {
	if !pluginIDPattern.MatchString(pluginID) {
		return fmt.Errorf("uninstall plugin: invalid plugin id %q", pluginID)
	}
	for _, table := range dropTables {
		if !tableNamePattern.MatchString(table) {
			return fmt.Errorf("uninstall plugin %q: invalid table name %q", pluginID, table)
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range dropTables {
		// No CASCADE: a table still referenced by another plugin's foreign key
		// must fail loudly instead of silently dragging data along.
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS "`+table+`"`); err != nil {
			return fmt.Errorf("uninstall plugin %q: drop table %s: %w", pluginID, table, err)
		}
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM sys_plugin_migration WHERE plugin_id = $1`, []any{pluginID}},
		{`DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE plugin_id = $1)`, []any{pluginID}},
		{`DELETE FROM sys_menu WHERE plugin_id = $1`, []any{pluginID}},
		{`DELETE FROM sys_plugin WHERE plugin_id = $1`, []any{pluginID}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return fmt.Errorf("uninstall plugin %q: %w", pluginID, err)
		}
	}
	return tx.Commit()
}

// SetPluginError records why a plugin is not running.
func (r *PluginRepository) SetPluginError(ctx context.Context, pluginID, message string) error {
	if len(message) > maxLastErrorLen {
		message = message[:maxLastErrorLen]
	}
	return r.queries.SetPluginError(ctx, sqlcstore.SetPluginErrorParams{
		PluginID:  pluginID,
		LastError: message,
	})
}

// AppliedMigrations returns version → checksum for one plugin.
func (r *PluginRepository) AppliedMigrations(ctx context.Context, pluginID string) (map[string]string, error) {
	rows, err := r.queries.ListPluginMigrations(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	applied := make(map[string]string, len(rows))
	for _, row := range rows {
		applied[row.Version] = row.Checksum
	}
	return applied, nil
}

// ApplyMigration runs one migration file and records it in the SAME
// transaction, so the presence of the bookkeeping row means "committed".
//
// The SQL is executed with no bind parameters, which makes pgx use the simple
// protocol and therefore accept multiple statements per file. Migration files
// must not contain $1-style placeholders.
func (r *PluginRepository) ApplyMigration(ctx context.Context, pluginID, version, name, checksum, script string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("execute migration %s/%s: %w", pluginID, version, err)
	}
	if err := r.queries.WithTx(tx.Tx).RecordPluginMigration(ctx, sqlcstore.RecordPluginMigrationParams{
		PluginID: pluginID,
		Version:  version,
		Name:     name,
		Checksum: checksum,
	}); err != nil {
		return fmt.Errorf("record migration %s/%s: %w", pluginID, version, err)
	}
	return tx.Commit()
}

// AcquireMigrationLock takes the session-level advisory lock and returns the
// release function. Session level (not transaction level) because each
// migration file commits on its own; PostgreSQL drops the lock automatically
// if the process dies, because the dedicated connection closes.
func (r *PluginRepository) AcquireMigrationLock(ctx context.Context, timeout time.Duration) (func(), error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration lock connection: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		var acquired bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("try migration lock: %w", err)
		}
		if acquired {
			return func() {
				// context.Background: the lock must be released even when the
				// request that took it has already been cancelled.
				_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
				_ = conn.Close()
			}, nil
		}
		if time.Now().After(deadline) {
			_ = conn.Close()
			return nil, ErrMigrationBusy
		}
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// CountPluginMigrations is used by diagnostics/tests.
func (r *PluginRepository) CountPluginMigrations(ctx context.Context) (int64, error) {
	return r.queries.CountPluginMigrations(ctx)
}
