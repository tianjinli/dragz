package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

type updateConfigRequest struct {
	Value string `json:"value"`
}

type saveAuthProviderRequest struct {
	ProviderType     string  `json:"providerType"`
	Name             string  `json:"name"`
	Icon             string  `json:"icon"`
	ClientID         string  `json:"clientId"`
	ClientSecret     string  `json:"clientSecret"`
	AuthorizationURL string  `json:"authorizationUrl"`
	TokenURL         string  `json:"tokenUrl"`
	UserInfoURL      string  `json:"userInfoUrl"`
	IssuerURL        string  `json:"issuerUrl"`
	CallbackURL      string  `json:"callbackUrl"`
	Scopes           string  `json:"scopes"`
	AutoRegister     bool    `json:"autoRegister"`
	DefaultRoleID    *uint64 `json:"defaultRoleId"`
	BindingRule      string  `json:"bindingRule"`
	Status           int8    `json:"status"`
	Sort             int     `json:"sort"`
}

type saveStorageConfigRequest struct {
	Name              string `json:"name"`
	StorageType       string `json:"storageType"`
	Endpoint          string `json:"endpoint"`
	Bucket            string `json:"bucket"`
	Region            string `json:"region"`
	AccessKey         string `json:"accessKey"`
	SecretKey         string `json:"secretKey"`
	BasePath          string `json:"basePath"`
	PublicBaseURL     string `json:"publicBaseUrl"`
	UseHTTPS          bool   `json:"useHttps"`
	PrivateAccess     bool   `json:"privateAccess"`
	MaxFileSize       uint64 `json:"maxFileSize"`
	AllowedExtensions string `json:"allowedExtensions"`
	IsDefault         bool   `json:"isDefault"`
	Status            int8   `json:"status"`
}

func listConfigsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Auth.ListConfigs(r.Context(), r.URL.Query().Get("group"))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.config_list_failed")
			return
		}
		// Preset configs store label/description as i18n message IDs.
		ok(w, r, localizeList(r, items, "name", "description"))
	}
}

func updateConfigHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimSpace(pathvarValue(r, "key"))
		if key == "" {
			fail(w, r, http.StatusBadRequest, 40001, "errors.config_key_required")
			return
		}
		var req updateConfigRequest
		if !decode(w, r, &req) {
			return
		}
		metadata, err := ctx.Auth.ConfigMetadata(r.Context(), key)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.config_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.config_read_failed")
			return
		}
		if metadata.IsSecret && (req.Value == "" || req.Value == "********") {
			ok(w, r, map[string]bool{"success": true})
			return
		}
		if !validateConfigValue(metadata.ValueType, req.Value) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.config_type_mismatch")
			return
		}
		if key == "security.password.minLength" {
			length, parseErr := strconv.Atoi(strings.TrimSpace(req.Value))
			if parseErr != nil || length < 8 || length > 64 {
				fail(w, r, http.StatusBadRequest, 40001, "errors.password_min_length_invalid")
				return
			}
		}
		value := req.Value
		if metadata.IsSecret {
			value, err = ctx.Secrets.Encrypt(value)
			if err != nil {
				fail(w, r, http.StatusInternalServerError, 50000, "errors.config_encrypt_failed")
				return
			}
		}
		operatorID, _ := currentUserID(r.Context())
		if err := ctx.Auth.UpdateConfig(r.Context(), key, value, operatorID); err != nil {
			if strings.Contains(err.Error(), "readonly") {
				fail(w, r, http.StatusConflict, 40901, "errors.settings.readonly")
				return
			}
			fail(w, r, http.StatusInternalServerError, 50000, "errors.config_save_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listAuthProvidersHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Auth.ListAuthProviders(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_provider_list_failed")
			return
		}
		ok(w, r, items)
	}
}

func createAuthProviderHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveAuthProviderRequest
		if !decode(w, r, &req) || !validateAuthProvider(w, r, req, true) {
			return
		}
		secret, err := ctx.Secrets.Encrypt(req.ClientSecret)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_provider_encrypt_failed")
			return
		}
		operatorID, _ := currentUserID(r.Context())
		id, err := ctx.Auth.CreateAuthProvider(r.Context(), authProviderInput(req, secret), operatorID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_provider_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateAuthProviderHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_provider_id_invalid")
			return
		}
		var req saveAuthProviderRequest
		if !decode(w, r, &req) || !validateAuthProvider(w, r, req, false) {
			return
		}
		keepSecret := req.ClientSecret == "" || req.ClientSecret == "********"
		secret := ""
		var err error
		if !keepSecret {
			secret, err = ctx.Secrets.Encrypt(req.ClientSecret)
			if err != nil {
				fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_provider_encrypt_failed")
				return
			}
		}
		operatorID, _ := currentUserID(r.Context())
		if err := ctx.Auth.UpdateAuthProvider(r.Context(), id, authProviderInput(req, secret), keepSecret, operatorID); err != nil {
			writeSettingsError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteAuthProviderHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_provider_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteAuthProvider(r.Context(), id); err != nil {
			writeSettingsError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updateAuthProviderStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_provider_id_invalid")
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_status_invalid")
			return
		}
		if err := ctx.Auth.UpdateAuthProviderStatus(r.Context(), id, req.Status); err != nil {
			writeSettingsError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listStorageConfigsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Auth.ListStorageConfigs(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_config_load_failed")
			return
		}
		items = localizePreset(r, ctx.Auth, items,
			func(s store.StorageConfig) []string { return []string{"storage." + s.StorageType} },
			func(s *store.StorageConfig, v map[string]string) {
				if t, ok := v["storage."+s.StorageType]; ok {
					s.Name = t
				}
			})
		ok(w, r, items)
	}
}

func createStorageConfigHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveStorageConfigRequest
		if !decode(w, r, &req) || !validateStorageConfig(w, r, req, true) {
			return
		}
		accessKey, secretKey, err := encryptStorageSecrets(ctx, req.AccessKey, req.SecretKey)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_encrypt_failed")
			return
		}
		input := storageConfigInput(req, accessKey, secretKey)
		save, err := preparePresetSave(ctx.Auth, r, storagePresetEdit(0, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		operatorID, _ := currentUserID(r.Context())
		id, err := ctx.Auth.CreateStorageConfig(r.Context(), input, operatorID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_config_save_failed")
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

// storagePresetEdit describes a storage config save. Its entity key is built
// from the storage type rather than a code column, because the type is what the
// readers localize by.
func storagePresetEdit(id uint64, input store.SaveStorageConfigInput) presetTextEdit {
	return presetTextEdit{
		Namespace: "storage", Prefix: "storage", ID: id,
		Code: input.StorageType, Name: input.Name,
	}
}

func updateStorageConfigHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.storage_id_invalid")
			return
		}
		var req saveStorageConfigRequest
		if !decode(w, r, &req) || !validateStorageConfig(w, r, req, false) {
			return
		}
		keepAccessKey := req.AccessKey == "" || req.AccessKey == "********"
		keepSecretKey := req.SecretKey == "" || req.SecretKey == "********"
		accessKey, secretKey := "", ""
		var err error
		if !keepAccessKey || !keepSecretKey {
			accessKey, secretKey, err = encryptStorageSecrets(ctx, optionalSecret(req.AccessKey, keepAccessKey), optionalSecret(req.SecretKey, keepSecretKey))
			if err != nil {
				fail(w, r, http.StatusInternalServerError, 50000, "errors.storage_encrypt_failed")
				return
			}
		}
		operatorID, _ := currentUserID(r.Context())
		input := storageConfigInput(req, accessKey, secretKey)
		save, err := preparePresetSave(ctx.Auth, r, storagePresetEdit(id, input))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		input.Name = save.SourceName()
		if err := ctx.Auth.UpdateStorageConfig(r.Context(), id, input, keepAccessKey, keepSecretKey, operatorID); err != nil {
			writeSettingsError(w, r, err)
			return
		}
		if err := save.Apply(r.Context()); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.translation_save_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteStorageConfigHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.storage_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteStorageConfig(r.Context(), id); err != nil {
			writeSettingsError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func validateConfigValue(valueType, value string) bool {
	switch valueType {
	case "string", "encrypted_string":
		return true
	case "number":
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	case "boolean":
		_, err := strconv.ParseBool(value)
		return err == nil
	case "json":
		return json.Valid([]byte(value))
	default:
		return false
	}
}

func validateAuthProvider(w http.ResponseWriter, r *http.Request, req saveAuthProviderRequest, create bool) bool {
	if strings.TrimSpace(req.ProviderType) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.ClientID) == "" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_provider_fields_invalid")
		return false
	}
	if create && req.ClientSecret == "" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_client_secret_required")
		return false
	}
	for _, endpoint := range []struct {
		labelKey string
		value    string
	}{
		{labelKey: "subjects.oauth_authorization_url", value: req.AuthorizationURL},
		{labelKey: "subjects.oauth_token_url", value: req.TokenURL},
		{labelKey: "subjects.oauth_userinfo_url", value: req.UserInfoURL},
		{labelKey: "subjects.oauth_callback_url", value: req.CallbackURL},
	} {
		if !validOAuthURL(endpoint.value) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_endpoint_url_invalid", map[string]any{
				"Label": localized(r, endpoint.labelKey),
			})
			return false
		}
	}
	if strings.TrimSpace(req.IssuerURL) != "" && !validOAuthURL(req.IssuerURL) {
		fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_issuer_invalid")
		return false
	}
	if req.BindingRule != "manual" && req.BindingRule != "email" && req.BindingRule != "mobile" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_binding_invalid")
		return false
	}
	return req.Status == 0 || req.Status == 1
}

func validOAuthURL(value string) bool {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func authProviderInput(req saveAuthProviderRequest, encryptedSecret string) store.SaveAuthProviderInput {
	return store.SaveAuthProviderInput{
		ProviderType: strings.TrimSpace(req.ProviderType), Name: strings.TrimSpace(req.Name),
		Icon: strings.TrimSpace(req.Icon), ClientID: strings.TrimSpace(req.ClientID),
		ClientSecretEncrypted: encryptedSecret, AuthorizationURL: strings.TrimSpace(req.AuthorizationURL),
		TokenURL: strings.TrimSpace(req.TokenURL), UserInfoURL: strings.TrimSpace(req.UserInfoURL),
		IssuerURL: strings.TrimSpace(req.IssuerURL), CallbackURL: strings.TrimSpace(req.CallbackURL),
		Scopes: strings.TrimSpace(req.Scopes), AutoRegister: req.AutoRegister,
		DefaultRoleID: req.DefaultRoleID, BindingRule: req.BindingRule, Status: req.Status, Sort: req.Sort,
	}
}

func validateStorageConfig(w http.ResponseWriter, r *http.Request, req saveStorageConfigRequest, create bool) bool {
	if strings.TrimSpace(req.Name) == "" || (req.StorageType != "local" && req.StorageType != "s3") {
		fail(w, r, http.StatusBadRequest, 40001, "errors.storage_fields_invalid")
		return false
	}
	if req.StorageType == "local" && strings.TrimSpace(req.BasePath) == "" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.storage_local_path_required")
		return false
	}
	if req.StorageType == "s3" && (strings.TrimSpace(req.Endpoint) == "" || strings.TrimSpace(req.Bucket) == "") {
		fail(w, r, http.StatusBadRequest, 40001, "errors.storage_s3_endpoint_required")
		return false
	}
	if create && req.StorageType == "s3" && (req.AccessKey == "" || req.SecretKey == "") {
		fail(w, r, http.StatusBadRequest, 40001, "errors.storage_s3_keys_required")
		return false
	}
	if req.MaxFileSize == 0 {
		fail(w, r, http.StatusBadRequest, 40001, "errors.storage_max_size_invalid")
		return false
	}
	return req.Status == 0 || req.Status == 1
}

func storageConfigInput(req saveStorageConfigRequest, accessKey, secretKey string) store.SaveStorageConfigInput {
	return store.SaveStorageConfigInput{
		Name: strings.TrimSpace(req.Name), StorageType: req.StorageType, Endpoint: strings.TrimSpace(req.Endpoint),
		Bucket: strings.TrimSpace(req.Bucket), Region: strings.TrimSpace(req.Region),
		AccessKeyEncrypted: accessKey, SecretKeyEncrypted: secretKey, BasePath: strings.TrimSpace(req.BasePath),
		PublicBaseURL: strings.TrimSpace(req.PublicBaseURL), UseHTTPS: req.UseHTTPS,
		PrivateAccess: req.PrivateAccess, MaxFileSize: req.MaxFileSize,
		AllowedExtensions: strings.TrimSpace(req.AllowedExtensions), IsDefault: req.IsDefault, Status: req.Status,
	}
}

func encryptStorageSecrets(ctx *app.Context, accessKey, secretKey string) (string, string, error) {
	encryptedAccessKey, err := ctx.Secrets.Encrypt(accessKey)
	if err != nil {
		return "", "", err
	}
	encryptedSecretKey, err := ctx.Secrets.Encrypt(secretKey)
	return encryptedAccessKey, encryptedSecretKey, err
}

func optionalSecret(value string, keep bool) string {
	if keep {
		return ""
	}
	return value
}

func writeSettingsError(w http.ResponseWriter, r *http.Request, err error) {
	failHTTP(w, r, apierrors.MapSettings(err))
}
