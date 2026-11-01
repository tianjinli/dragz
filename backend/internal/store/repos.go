package store

import (
	"context"
	"database/sql"
	"time"

	"dragz-admin/backend/internal/menuperm"
)

// AuthStore is the persistence boundary used by logic/httpapi for auth/session flows.
// PostgreSQL is implemented by AuthRepository; other drivers should provide their own impl.
type AuthStore interface {
	CountUsers() (int64, error)
	FindUserByUsername(ctx context.Context, username string) (User, error)
	FindUserByID(ctx context.Context, userID uint64) (User, error)
	LoginSecuritySettings(ctx context.Context) (LoginSecuritySettings, error)
	RecordLoginFailure(ctx context.Context, username, ip, userAgent, reason string, lockUntil *time.Time) error
	RecordLoginSuccess(ctx context.Context, userID uint64, username, ip, userAgent string) error
	SaveRefreshToken(ctx context.Context, userID uint64, tokenHash, familyID, deviceID, deviceName, ip, userAgent string, expiresAt time.Time) error
	FindRefreshSession(ctx context.Context, tokenHash string) (RefreshSession, error)
	ForceLogoutUser(ctx context.Context, userID uint64) error
	FeatureEnabled(ctx context.Context, key string, defaultValue bool) (bool, error)
	UserRoleIDs(ctx context.Context, userID uint64) ([]uint64, error)
	UserRoles(ctx context.Context, userID uint64) ([]UserRole, error)
	DepartmentRef(ctx context.Context, departmentID sql.NullInt64) (NamedRef, error)
	PermissionCodes(ctx context.Context, userID uint64, superAdmin bool) ([]string, error)
	Menus(ctx context.Context, userID uint64, superAdmin bool) ([]Menu, error)
	ResolveUserDataScope(ctx context.Context, userID uint64) (DataScope, error)
}

// UserStore covers admin user CRUD (beyond AuthStore lookups).
type UserStore interface {
	ListUsers(ctx context.Context, filter UserListFilter) ([]UserListItem, int64, error)
	UserDetail(ctx context.Context, userID uint64) (UserDetail, error)
	CreateUser(ctx context.Context, input SaveUserInput) (uint64, error)
	UpdateUser(ctx context.Context, userID uint64, input SaveUserInput) error
	DeleteUser(ctx context.Context, userID uint64) error
	ResetUserPassword(ctx context.Context, userID uint64, passwordHash string) error
	UpdateSelfProfile(ctx context.Context, userID uint64, displayName, email, mobile string) error
	ChangeSelfPassword(ctx context.Context, userID uint64, passwordHash string) error
}

// JobStore covers scheduled jobs and job logs.
type JobStore interface {
	ListJobs(ctx context.Context) ([]ScheduledJob, error)
	JobByID(ctx context.Context, jobID uint64) (ScheduledJob, error)
	CreateJob(ctx context.Context, input SaveJobInput) (uint64, error)
	UpdateJob(ctx context.Context, jobID uint64, input SaveJobInput) error
	DeleteJob(ctx context.Context, jobID uint64) error
	ListEnabledJobs(ctx context.Context) ([]ScheduledJob, error)
	UpdateJobNextRunAt(ctx context.Context, jobID uint64, next time.Time) error
	StartJobLog(ctx context.Context, jobID uint64, triggerType string, attempt int) (uint64, error)
	FinishJobLog(ctx context.Context, jobID, logID uint64, success bool, output, errorMessage string) error
	ListJobLogs(ctx context.Context, jobID uint64, page, pageSize int) ([]JobLog, int64, error)
}

// SettingsStore covers system config, auth providers, and storage configs.
type SettingsStore interface {
	ListConfigs(ctx context.Context, group string) ([]ConfigItem, error)
	ConfigMetadata(ctx context.Context, key string) (ConfigItem, error)
	UpdateConfig(ctx context.Context, key, value string, operatorID uint64) error
	ListAuthProviders(ctx context.Context) ([]AuthProvider, error)
	CreateAuthProvider(ctx context.Context, input SaveAuthProviderInput, operatorID uint64) (uint64, error)
	UpdateAuthProvider(ctx context.Context, providerID uint64, input SaveAuthProviderInput, keepSecret bool, operatorID uint64) error
	DeleteAuthProvider(ctx context.Context, providerID uint64) error
	ListStorageConfigs(ctx context.Context) ([]StorageConfig, error)
	DefaultStorageConfig(ctx context.Context) (StorageConfig, error)
	CreateStorageConfig(ctx context.Context, input SaveStorageConfigInput, operatorID uint64) (uint64, error)
	UpdateStorageConfig(ctx context.Context, storageID uint64, input SaveStorageConfigInput, keepAccessKey, keepSecretKey bool, operatorID uint64) error
	DeleteStorageConfig(ctx context.Context, storageID uint64) error
}

// PluginStore covers the plugin catalog, its runtime state and the
// bookkeeping for plugin migrations.
type PluginStore interface {
	Bootstrap(ctx context.Context) error
	Seed(ctx context.Context) error
	SyncCatalog(ctx context.Context, entries []PluginCatalogEntry) error
	ListPlugins(ctx context.Context) ([]PluginRecord, error)
	SetPluginEnabled(ctx context.Context, pluginID string, enabled bool) (bool, error)
	SetPluginError(ctx context.Context, pluginID, message string) error
	AppliedMigrations(ctx context.Context, pluginID string) (map[string]string, error)
	ApplyMigration(ctx context.Context, pluginID, version, name, checksum, script string) error
	AcquireMigrationLock(ctx context.Context, timeout time.Duration) (func(), error)
	// UninstallPlugin removes a plugin's tables (dropTables), its menu rows and
	// role grants, its migration bookkeeping and its catalog row, in one
	// transaction. Table names must be plain identifiers; anything else is
	// rejected before DDL runs.
	UninstallPlugin(ctx context.Context, pluginID string, dropTables []string) error
	// MenuCodeIndex and SyncPermissionMenus keep sys_menu buttons aligned with
	// the compiled route table.
	MenuCodeIndex(ctx context.Context) (map[string]uint64, error)
	SyncPermissionMenus(ctx context.Context, buttons []menuperm.Button) error
}

// Ensure AuthRepository satisfies the expanded store interfaces.
var (
	_ AuthStore     = (*AuthRepository)(nil)
	_ UserStore     = (*AuthRepository)(nil)
	_ JobStore      = (*AuthRepository)(nil)
	_ SettingsStore = (*AuthRepository)(nil)
	_ PluginStore   = (*PluginRepository)(nil)
)
