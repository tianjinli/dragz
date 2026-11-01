package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// ConfigItem is one sys_config row. A secret value is masked before it leaves the repository.
type ConfigItem struct {
	ID           uint64    `json:"id"`
	Group        string    `json:"group"`
	Key          string    `json:"key"`
	Name         string    `json:"name"`
	ValueType    string    `json:"valueType"`
	Value        string    `json:"value"`
	DefaultValue string    `json:"defaultValue"`
	IsSecret     bool      `json:"isSecret"`
	IsReadonly   bool      `json:"isReadonly"`
	EffectMode   string    `json:"effectMode"`
	Description  string    `json:"description"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AuthProvider is one sys_auth_provider row. ClientSecret holds ciphertext, and the list query masks it.
type AuthProvider struct {
	ID               uint64    `json:"id"`
	ProviderType     string    `json:"providerType"`
	Name             string    `json:"name"`
	Icon             string    `json:"icon"`
	ClientID         string    `json:"clientId"`
	ClientSecret     string    `json:"clientSecret"`
	AuthorizationURL string    `json:"authorizationUrl"`
	TokenURL         string    `json:"tokenUrl"`
	UserInfoURL      string    `json:"userInfoUrl"`
	IssuerURL        string    `json:"issuerUrl"`
	CallbackURL      string    `json:"callbackUrl"`
	Scopes           string    `json:"scopes"`
	AutoRegister     bool      `json:"autoRegister"`
	DefaultRoleID    *uint64   `json:"defaultRoleId"`
	BindingRule      string    `json:"bindingRule"`
	Status           int8      `json:"status"`
	Sort             int       `json:"sort"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

// SaveAuthProviderInput is the payload shared by create and update. The secret arrives already encrypted.
type SaveAuthProviderInput struct {
	ProviderType          string
	Name                  string
	Icon                  string
	ClientID              string
	ClientSecretEncrypted string
	AuthorizationURL      string
	TokenURL              string
	UserInfoURL           string
	IssuerURL             string
	CallbackURL           string
	Scopes                string
	AutoRegister          bool
	DefaultRoleID         *uint64
	BindingRule           string
	Status                int8
	Sort                  int
}

// StorageConfig is one sys_storage_config row. The access key and secret hold ciphertext, and the list query masks them.
type StorageConfig struct {
	ID                uint64    `json:"id"`
	Name              string    `json:"name"`
	StorageType       string    `json:"storageType"`
	Endpoint          string    `json:"endpoint"`
	Bucket            string    `json:"bucket"`
	Region            string    `json:"region"`
	AccessKey         string    `json:"accessKey"`
	SecretKey         string    `json:"secretKey"`
	BasePath          string    `json:"basePath"`
	PublicBaseURL     string    `json:"publicBaseUrl"`
	UseHTTPS          bool      `json:"useHttps"`
	PrivateAccess     bool      `json:"privateAccess"`
	MaxFileSize       uint64    `json:"maxFileSize"`
	AllowedExtensions string    `json:"allowedExtensions"`
	IsDefault         bool      `json:"isDefault"`
	Status            int8      `json:"status"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// SaveStorageConfigInput is the payload shared by create and update. Both keys arrive already encrypted.
type SaveStorageConfigInput struct {
	Name               string
	StorageType        string
	Endpoint           string
	Bucket             string
	Region             string
	AccessKeyEncrypted string
	SecretKeyEncrypted string
	BasePath           string
	PublicBaseURL      string
	UseHTTPS           bool
	PrivateAccess      bool
	MaxFileSize        uint64
	AllowedExtensions  string
	IsDefault          bool
	Status             int8
}

// LoginSecuritySettings is the resolved login policy: the three security.* config keys with their defaults applied.
type LoginSecuritySettings struct {
	MaxAttempts       int
	LockMinutes       int
	CaptchaEnabled    bool
	PasswordMinLength int
}

func mapConfigRow(id int64, group, key, name, valueType, value, defaultValue string, isSecret, isReadonly int16, effectMode, description string, updatedAt time.Time) ConfigItem {
	return ConfigItem{
		ID: uint64(id), Group: group, Key: key, Name: name, ValueType: valueType,
		Value: value, DefaultValue: defaultValue, IsSecret: smallIntBool(isSecret),
		IsReadonly: smallIntBool(isReadonly), EffectMode: effectMode, Description: description, UpdatedAt: updatedAt,
	}
}

// LoginSecuritySettings reads the login policy, falling back to the built-in defaults when a key is missing or out of range.
func (r *AuthRepository) LoginSecuritySettings(ctx context.Context) (LoginSecuritySettings, error) {
	settings := LoginSecuritySettings{MaxAttempts: 5, LockMinutes: 30, CaptchaEnabled: false, PasswordMinLength: 8}
	if value, err := r.configIntValue(ctx, "security.login.maxAttempts"); err != nil {
		return settings, err
	} else if value > 0 {
		settings.MaxAttempts = value
	}
	if value, err := r.configIntValue(ctx, "security.login.lockMinutes"); err != nil {
		return settings, err
	} else if value > 0 {
		settings.LockMinutes = value
	}
	if value, err := r.configIntValue(ctx, "security.password.minLength"); err != nil {
		return settings, err
	} else if value >= 8 && value <= 64 {
		settings.PasswordMinLength = value
	}
	if value, err := r.CaptchaEnabled(ctx); err != nil {
		return settings, err
	} else {
		settings.CaptchaEnabled = value
	}
	return settings, nil
}

// CaptchaEnabled reports whether the login form has to solve a captcha. The legacy afterFailures key is honoured so an upgraded database keeps its behaviour.
func (r *AuthRepository) CaptchaEnabled(ctx context.Context) (bool, error) {
	value, err := r.queries.GetConfigValue(ctx, "security.captcha.enabled")
	if errors.Is(err, sql.ErrNoRows) {
		_, legacyErr := r.queries.GetConfigValue(ctx, "security.captcha.afterFailures")
		if errors.Is(legacyErr, sql.ErrNoRows) {
			return false, nil
		}
		if legacyErr != nil {
			return false, legacyErr
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, nil
	}
	return parsed, nil
}

func (r *AuthRepository) configIntValue(ctx context.Context, key string) (int, error) {
	value, err := r.queries.GetConfigValue(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

// FeatureEnabled reads a boolean feature flag, returning defaultValue when the key is absent.
func (r *AuthRepository) FeatureEnabled(ctx context.Context, key string, defaultValue bool) (bool, error) {
	value, err := r.queries.GetFeatureConfigValue(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultValue, nil
	}
	if err != nil {
		return false, err
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, err
	}
	return parsed, nil
}

// ListConfigs returns the configuration items of one group, or of every group when the group is empty.
func (r *AuthRepository) ListConfigs(ctx context.Context, group string) ([]ConfigItem, error) {
	group = strings.TrimSpace(group)
	var items []ConfigItem
	if group == "" {
		rows, err := r.queries.ListAllConfigs(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			item := mapConfigRow(row.ID, row.ConfigGroup, row.ConfigKey, row.Name, row.ValueType, row.ConfigValue, row.DefaultValue, row.IsSecret, row.IsReadonly, row.EffectMode, row.Description, row.UpdatedAt)
			if item.IsSecret && item.Value != "" {
				item.Value = "********"
			}
			items = append(items, item)
		}
	} else {
		rows, err := r.queries.ListConfigsByGroup(ctx, group)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			item := mapConfigRow(row.ID, row.ConfigGroup, row.ConfigKey, row.Name, row.ValueType, row.ConfigValue, row.DefaultValue, row.IsSecret, row.IsReadonly, row.EffectMode, row.Description, row.UpdatedAt)
			if item.IsSecret && item.Value != "" {
				item.Value = "********"
			}
			items = append(items, item)
		}
	}
	if items == nil {
		items = []ConfigItem{}
	}
	return items, nil
}

// ConfigMetadata loads one configuration item including its type and readonly flag, which the editor needs before it can validate a value.
func (r *AuthRepository) ConfigMetadata(ctx context.Context, key string) (ConfigItem, error) {
	row, err := r.queries.GetConfigByKey(ctx, key)
	if err != nil {
		return ConfigItem{}, err
	}
	return mapConfigRow(row.ID, row.ConfigGroup, row.ConfigKey, row.Name, row.ValueType, row.ConfigValue, row.DefaultValue, row.IsSecret, row.IsReadonly, row.EffectMode, row.Description, row.UpdatedAt), nil
}

// UpdateConfig writes a value and records the before/after pair in the config log in one transaction. Secret values are masked in the log.
func (r *AuthRepository) UpdateConfig(ctx context.Context, key, value string, operatorID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	beforeRow, err := q.GetConfigByKeyForUpdate(ctx, key)
	if err != nil {
		return err
	}
	before := mapConfigRow(beforeRow.ID, beforeRow.ConfigGroup, beforeRow.ConfigKey, beforeRow.Name, beforeRow.ValueType, beforeRow.ConfigValue, beforeRow.DefaultValue, beforeRow.IsSecret, beforeRow.IsReadonly, beforeRow.EffectMode, beforeRow.Description, beforeRow.UpdatedAt)
	if before.IsReadonly {
		return ErrReadonly
	}
	if err := q.UpdateConfigValue(ctx, sqlcstore.UpdateConfigValueParams{ConfigValue: value, ConfigKey: key}); err != nil {
		return err
	}
	beforeJSON, _ := json.Marshal(map[string]any{"value": auditConfigValue(before.Value, before.IsSecret)})
	afterJSON, _ := json.Marshal(map[string]any{"value": auditConfigValue(value, before.IsSecret)})
	if err := q.InsertConfigLog(ctx, sqlcstore.InsertConfigLogParams{
		ConfigKey: key, OperatorID: operatorID,
		BeforeJson: jsonNullRaw(beforeJSON), AfterJson: jsonNullRaw(afterJSON),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func mapAuthProviderRow(id int64, providerType, name, icon, clientID, secret, authURL, tokenURL, userInfoURL, issuerURL, callbackURL, scopes string, autoRegister int16, roleID sql.NullInt64, bindingRule string, status int16, sort int32, updatedAt time.Time) AuthProvider {
	item := AuthProvider{
		ID: uint64(id), ProviderType: providerType, Name: name, Icon: icon, ClientID: clientID,
		ClientSecret: secret, AuthorizationURL: authURL, TokenURL: tokenURL, UserInfoURL: userInfoURL,
		IssuerURL: issuerURL, CallbackURL: callbackURL, Scopes: scopes, AutoRegister: smallIntBool(autoRegister),
		DefaultRoleID: nullInt64Ptr(roleID), BindingRule: bindingRule, Status: int8(status), Sort: int(sort), UpdatedAt: updatedAt,
	}
	return item
}

// ListAuthProviders returns every third-party login provider with its secret masked.
func (r *AuthRepository) ListAuthProviders(ctx context.Context) ([]AuthProvider, error) {
	rows, err := r.queries.ListAuthProviders(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]AuthProvider, 0, len(rows))
	for _, row := range rows {
		item := mapAuthProviderRow(row.ID, row.ProviderType, row.Name, row.Icon, row.ClientID, row.ClientSecretEncrypted,
			row.AuthorizationUrl, row.TokenUrl, row.UserInfoUrl, row.IssuerUrl, row.CallbackUrl, row.Scopes,
			row.AutoRegister, row.DefaultRoleID, row.BindingRule, row.Status, row.Sort, row.UpdatedAt)
		if item.ClientSecret != "" {
			item.ClientSecret = "********"
		}
		items = append(items, item)
	}
	return items, nil
}

// AuthProviderSecret returns the encrypted client secret, which only the OAuth exchange needs.
func (r *AuthRepository) AuthProviderSecret(ctx context.Context, providerID uint64) (string, error) {
	return r.queries.GetAuthProviderSecret(ctx, int64(providerID))
}

// CreateAuthProvider inserts a third-party login provider.
func (r *AuthRepository) CreateAuthProvider(ctx context.Context, input SaveAuthProviderInput, operatorID uint64) (uint64, error) {
	id, err := r.queries.CreateAuthProvider(ctx, sqlcstore.CreateAuthProviderParams{
		ProviderType: input.ProviderType, Name: input.Name, Icon: input.Icon, ClientID: input.ClientID,
		ClientSecretEncrypted: input.ClientSecretEncrypted, AuthorizationUrl: input.AuthorizationURL,
		TokenUrl: input.TokenURL, UserInfoUrl: input.UserInfoURL, IssuerUrl: input.IssuerURL,
		CallbackUrl: input.CallbackURL, Scopes: input.Scopes, AutoRegister: boolSmallInt16(input.AutoRegister),
		DefaultRoleID: ptrUint64Null(input.DefaultRoleID), BindingRule: input.BindingRule,
		Status: int16(input.Status), Sort: int32(input.Sort),
	})
	return uint64(id), err
}

// UpdateAuthProvider rewrites a provider. keepSecret leaves the stored ciphertext alone when the form submitted the mask instead of a new secret.
func (r *AuthRepository) UpdateAuthProvider(ctx context.Context, providerID uint64, input SaveAuthProviderInput, keepSecret bool, operatorID uint64) error {
	var affected int64
	var err error
	base := sqlcstore.UpdateAuthProviderParams{
		ProviderType: input.ProviderType, Name: input.Name, Icon: input.Icon, ClientID: input.ClientID,
		AuthorizationUrl: input.AuthorizationURL, TokenUrl: input.TokenURL, UserInfoUrl: input.UserInfoURL,
		IssuerUrl: input.IssuerURL, CallbackUrl: input.CallbackURL, Scopes: input.Scopes,
		AutoRegister: boolSmallInt16(input.AutoRegister), DefaultRoleID: ptrUint64Null(input.DefaultRoleID),
		BindingRule: input.BindingRule, Status: int16(input.Status), Sort: int32(input.Sort), ID: int64(providerID),
	}
	if keepSecret {
		affected, err = r.queries.UpdateAuthProvider(ctx, base)
	} else {
		affected, err = r.queries.UpdateAuthProviderWithSecret(ctx, sqlcstore.UpdateAuthProviderWithSecretParams{
			ProviderType: base.ProviderType, Name: base.Name, Icon: base.Icon, ClientID: base.ClientID,
			AuthorizationUrl: base.AuthorizationUrl, TokenUrl: base.TokenUrl, UserInfoUrl: base.UserInfoUrl,
			IssuerUrl: base.IssuerUrl, CallbackUrl: base.CallbackUrl, Scopes: base.Scopes,
			AutoRegister: base.AutoRegister, DefaultRoleID: base.DefaultRoleID, BindingRule: base.BindingRule,
			Status: base.Status, Sort: base.Sort, ClientSecretEncrypted: input.ClientSecretEncrypted, ID: base.ID,
		})
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// UpdateAuthProviderStatus enables or disables a provider.
func (r *AuthRepository) UpdateAuthProviderStatus(ctx context.Context, providerID uint64, status int8) error {
	affected, err := r.queries.UpdateAuthProviderStatus(ctx, sqlcstore.UpdateAuthProviderStatusParams{
		Status: int16(status), ID: int64(providerID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// DeleteAuthProvider soft deletes a provider that no user identity references.
func (r *AuthRepository) DeleteAuthProvider(ctx context.Context, providerID uint64) error {
	identities, err := r.queries.CountUserIdentitiesByProvider(ctx, providerID)
	if err != nil {
		return err
	}
	if identities > 0 {
		return ErrInUse
	}
	affected, err := r.queries.SoftDeleteAuthProvider(ctx, int64(providerID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

func mapStorageRow(id int64, name, storageType, endpoint, bucket, region, accessKey, secretKey, basePath, publicBaseURL string, useHTTPS, privateAccess int16, maxFileSize uint64, allowedExtensions string, isDefault, status int16, updatedAt time.Time) StorageConfig {
	return StorageConfig{
		ID: uint64(id), Name: name, StorageType: storageType, Endpoint: endpoint, Bucket: bucket, Region: region,
		AccessKey: accessKey, SecretKey: secretKey, BasePath: basePath, PublicBaseURL: publicBaseURL,
		UseHTTPS: smallIntBool(useHTTPS), PrivateAccess: smallIntBool(privateAccess), MaxFileSize: maxFileSize,
		AllowedExtensions: allowedExtensions, IsDefault: smallIntBool(isDefault), Status: int8(status), UpdatedAt: updatedAt,
	}
}

// ListStorageConfigs returns every storage backend with both keys masked.
func (r *AuthRepository) ListStorageConfigs(ctx context.Context) ([]StorageConfig, error) {
	rows, err := r.queries.ListStorageConfigs(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]StorageConfig, 0, len(rows))
	for _, row := range rows {
		item := mapStorageRow(row.ID, row.Name, row.StorageType, row.Endpoint, row.Bucket, row.Region,
			row.AccessKeyEncrypted, row.SecretKeyEncrypted, row.BasePath, row.PublicBaseUrl,
			row.UseHttps, row.PrivateAccess, row.MaxFileSize, row.AllowedExtensions, row.IsDefault, row.Status, row.UpdatedAt)
		if item.AccessKey != "" {
			item.AccessKey = "********"
		}
		if item.SecretKey != "" {
			item.SecretKey = "********"
		}
		items = append(items, item)
	}
	return items, nil
}

// StorageSecrets returns the encrypted access key and secret of one backend.
func (r *AuthRepository) StorageSecrets(ctx context.Context, storageID uint64) (string, string, error) {
	row, err := r.queries.GetStorageSecrets(ctx, int64(storageID))
	return row.AccessKeyEncrypted.String, row.SecretKeyEncrypted.String, err
}

// StorageConfigByID loads one backend including its encrypted keys.
func (r *AuthRepository) StorageConfigByID(ctx context.Context, storageID uint64) (StorageConfig, error) {
	row, err := r.queries.GetStorageConfigByID(ctx, int64(storageID))
	if err != nil {
		return StorageConfig{}, err
	}
	return mapStorageRow(row.ID, row.Name, row.StorageType, row.Endpoint, row.Bucket, row.Region,
		row.AccessKeyEncrypted, row.SecretKeyEncrypted, row.BasePath, row.PublicBaseUrl,
		row.UseHttps, row.PrivateAccess, row.MaxFileSize, row.AllowedExtensions, row.IsDefault, row.Status, row.UpdatedAt), nil
}

// DefaultStorageConfig loads the backend new uploads go to.
func (r *AuthRepository) DefaultStorageConfig(ctx context.Context) (StorageConfig, error) {
	row, err := r.queries.GetDefaultStorageConfig(ctx)
	if err != nil {
		return StorageConfig{}, err
	}
	return mapStorageRow(row.ID, row.Name, row.StorageType, row.Endpoint, row.Bucket, row.Region,
		row.AccessKeyEncrypted, row.SecretKeyEncrypted, row.BasePath, row.PublicBaseUrl,
		row.UseHttps, row.PrivateAccess, row.MaxFileSize, row.AllowedExtensions, row.IsDefault, row.Status, row.UpdatedAt), nil
}

// CreateStorageConfig inserts a backend. Setting it as default clears the flag on every other backend in the same transaction, so exactly one default can exist.
func (r *AuthRepository) CreateStorageConfig(ctx context.Context, input SaveStorageConfigInput, operatorID uint64) (uint64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if input.IsDefault {
		if err := q.ClearDefaultStorageConfigs(ctx); err != nil {
			return 0, err
		}
	}
	id, err := q.CreateStorageConfig(ctx, sqlcstore.CreateStorageConfigParams{
		Name: input.Name, StorageType: input.StorageType, Endpoint: input.Endpoint, Bucket: input.Bucket,
		Region: input.Region, AccessKeyEncrypted: optionalString(input.AccessKeyEncrypted),
		SecretKeyEncrypted: optionalString(input.SecretKeyEncrypted), BasePath: input.BasePath,
		PublicBaseUrl: input.PublicBaseURL, UseHttps: boolSmallInt16(input.UseHTTPS),
		PrivateAccess: boolSmallInt16(input.PrivateAccess), MaxFileSize: input.MaxFileSize,
		AllowedExtensions: optionalString(input.AllowedExtensions), IsDefault: boolSmallInt16(input.IsDefault),
		Status: int16(input.Status),
	})
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// UpdateStorageConfig rewrites a backend. The two keep flags let the caller preserve a key the form submitted as a mask, and promoting it to default demotes the previous one in the same transaction.
func (r *AuthRepository) UpdateStorageConfig(ctx context.Context, storageID uint64, input SaveStorageConfigInput, keepAccessKey, keepSecretKey bool, operatorID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if input.IsDefault {
		if err := q.ClearOtherDefaultStorageConfigs(ctx, int64(storageID)); err != nil {
			return err
		}
	}
	var affected int64
	base := sqlcstore.UpdateStorageConfigParams{
		Name: input.Name, StorageType: input.StorageType, Endpoint: input.Endpoint, Bucket: input.Bucket,
		Region: input.Region, BasePath: input.BasePath, PublicBaseUrl: input.PublicBaseURL,
		UseHttps: boolSmallInt16(input.UseHTTPS), PrivateAccess: boolSmallInt16(input.PrivateAccess),
		MaxFileSize: input.MaxFileSize, AllowedExtensions: optionalString(input.AllowedExtensions),
		IsDefault: boolSmallInt16(input.IsDefault), Status: int16(input.Status), ID: int64(storageID),
	}
	switch {
	case !keepAccessKey && !keepSecretKey:
		affected, err = q.UpdateStorageConfigWithBothKeys(ctx, sqlcstore.UpdateStorageConfigWithBothKeysParams{
			Name: base.Name, StorageType: base.StorageType, Endpoint: base.Endpoint, Bucket: base.Bucket,
			Region: base.Region, BasePath: base.BasePath, PublicBaseUrl: base.PublicBaseUrl,
			UseHttps: base.UseHttps, PrivateAccess: base.PrivateAccess, MaxFileSize: base.MaxFileSize,
			AllowedExtensions: base.AllowedExtensions, IsDefault: base.IsDefault, Status: base.Status,
			AccessKeyEncrypted: optionalString(input.AccessKeyEncrypted),
			SecretKeyEncrypted: optionalString(input.SecretKeyEncrypted), ID: base.ID,
		})
	case !keepAccessKey:
		affected, err = q.UpdateStorageConfigWithAccessKey(ctx, sqlcstore.UpdateStorageConfigWithAccessKeyParams{
			Name: base.Name, StorageType: base.StorageType, Endpoint: base.Endpoint, Bucket: base.Bucket,
			Region: base.Region, BasePath: base.BasePath, PublicBaseUrl: base.PublicBaseUrl,
			UseHttps: base.UseHttps, PrivateAccess: base.PrivateAccess, MaxFileSize: base.MaxFileSize,
			AllowedExtensions: base.AllowedExtensions, IsDefault: base.IsDefault, Status: base.Status,
			AccessKeyEncrypted: optionalString(input.AccessKeyEncrypted), ID: base.ID,
		})
	case !keepSecretKey:
		affected, err = q.UpdateStorageConfigWithSecretKey(ctx, sqlcstore.UpdateStorageConfigWithSecretKeyParams{
			Name: base.Name, StorageType: base.StorageType, Endpoint: base.Endpoint, Bucket: base.Bucket,
			Region: base.Region, BasePath: base.BasePath, PublicBaseUrl: base.PublicBaseUrl,
			UseHttps: base.UseHttps, PrivateAccess: base.PrivateAccess, MaxFileSize: base.MaxFileSize,
			AllowedExtensions: base.AllowedExtensions, IsDefault: base.IsDefault, Status: base.Status,
			SecretKeyEncrypted: optionalString(input.SecretKeyEncrypted), ID: base.ID,
		})
	default:
		affected, err = q.UpdateStorageConfig(ctx, base)
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return tx.Commit()
}

// DeleteStorageConfig soft deletes a backend. The default backend and any backend still holding files are refused with ErrDefaultOrInUse.
func (r *AuthRepository) DeleteStorageConfig(ctx context.Context, storageID uint64) error {
	checks, err := r.queries.GetStorageDeleteChecks(ctx, int64(storageID))
	if err != nil {
		return err
	}
	if smallIntBool(checks.IsDefault) || checks.FileCount > 0 {
		return ErrDefaultOrInUse
	}
	affected, err := r.queries.SoftDeleteStorageConfig(ctx, int64(storageID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

func auditConfigValue(value string, secret bool) string {
	if secret && value != "" {
		return "********"
	}
	return value
}
