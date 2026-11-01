package httpapi

import (
	"net/http"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/plugin"
)

// Routes is the declarative route table of the system module.
//
// P0: handlers still live in this package; plugins/system re-exports these
// declarations. P6 moves the handlers here wholesale.
func Routes() []plugin.Route {
	return []plugin.Route{
		// ---- public ----
		{Method: http.MethodPost, Path: "/api/auth/login", Public: true, Handler: loginHandler},
		{Method: http.MethodGet, Path: "/api/auth/captcha", Public: true, Handler: captchaIssueHandler},
		{Method: http.MethodGet, Path: "/api/auth/login-challenge", Public: true, Handler: loginChallengeHandler},
		{Method: http.MethodPost, Path: "/api/auth/refresh", Public: true, Handler: refreshHandler},
		{Method: http.MethodGet, Path: "/api/auth/providers", Public: true, Handler: publicAuthProvidersHandler},
		{Method: http.MethodGet, Path: "/api/auth/oauth/:id/start", Public: true, Handler: startOAuthHandler},
		{Method: http.MethodGet, Path: "/api/auth/oauth/:id/callback", Public: true, Handler: oauthCallbackHandler},
		{Method: http.MethodPost, Path: "/api/auth/oauth/exchange", Public: true, Handler: exchangeOAuthHandler},
		{Method: http.MethodGet, Path: "/api/health", Public: true, Handler: healthHandlerAdapter},
		{Method: http.MethodGet, Path: "/api/ready", Public: true, Handler: readyHandler},
		{Method: http.MethodGet, Path: "/api/system/files/:id/content", Public: true, Handler: publicFileContentHandler},

		// ---- auth / profile (login only) ----
		{Method: http.MethodPost, Path: "/api/auth/logout", Handler: logoutHandler},
		{Method: http.MethodGet, Path: "/api/auth/profile", Handler: profileHandler},
		{Method: http.MethodPut, Path: "/api/auth/profile", Handler: updateProfileHandler},
		{Method: http.MethodPost, Path: "/api/auth/change-password", Handler: changePasswordHandler},
		{Method: http.MethodGet, Path: "/api/auth/codes", Handler: permissionCodesHandler},
		{Method: http.MethodGet, Path: "/api/auth/menus", Handler: menusHandler},
		{Method: http.MethodGet, Path: "/api/dashboard", Handler: dashboardHandler},

		// ---- users ----
		{Method: http.MethodGet, Path: "/api/system/users", Permission: "system:user:query", Handler: listUsersHandler},
		{Method: http.MethodGet, Path: "/api/system/users/:id", Permission: "system:user:query", Handler: getUserHandler},
		{Method: http.MethodPost, Path: "/api/system/users", Permission: "system:user:create", Handler: createUserHandler},
		{Method: http.MethodPut, Path: "/api/system/users/:id", Permission: "system:user:update", Handler: updateUserHandler},
		{Method: http.MethodPut, Path: "/api/system/users/:id/status", Permission: "system:user:update", Handler: updateUserStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/users/:id", Permission: "system:user:delete", Handler: deleteUserHandler},
		{Method: http.MethodPost, Path: "/api/system/users/:id/reset-password", Permission: "system:user:reset-password", Handler: resetUserPasswordHandler},
		{Method: http.MethodPost, Path: "/api/system/users/:id/logout", Permission: "system:user:force-logout", Handler: forceLogoutUserHandler},

		// ---- departments ----
		{Method: http.MethodGet, Path: "/api/system/departments", Permission: "system:department:query", Handler: listDepartmentsHandler},
		{Method: http.MethodPost, Path: "/api/system/departments", Permission: "system:department:create", Handler: createDepartmentHandler},
		{Method: http.MethodPut, Path: "/api/system/departments/:id", Permission: "system:department:update", Handler: updateDepartmentHandler},
		{Method: http.MethodPut, Path: "/api/system/departments/:id/status", Permission: "system:department:update", Handler: updateDepartmentStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/departments/:id", Permission: "system:department:delete", Handler: deleteDepartmentHandler},

		// ---- positions ----
		{Method: http.MethodGet, Path: "/api/system/positions", Permission: "system:position:query", Handler: listPositionsHandler},
		{Method: http.MethodPost, Path: "/api/system/positions", Permission: "system:position:create", Handler: createPositionHandler},
		{Method: http.MethodPut, Path: "/api/system/positions/:id", Permission: "system:position:update", Handler: updatePositionHandler},
		{Method: http.MethodPut, Path: "/api/system/positions/:id/status", Permission: "system:position:update", Handler: updatePositionStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/positions/:id", Permission: "system:position:delete", Handler: deletePositionHandler},

		// ---- roles ----
		{Method: http.MethodGet, Path: "/api/system/roles", Permission: "system:role:query", Handler: listRolesHandler},
		{Method: http.MethodPost, Path: "/api/system/roles", Permission: "system:role:create", Handler: createRoleHandler},
		{Method: http.MethodPut, Path: "/api/system/roles/:id", Permission: "system:role:update", Handler: updateRoleHandler},
		{Method: http.MethodPut, Path: "/api/system/roles/:id/status", Permission: "system:role:update", Handler: updateRoleStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/roles/:id", Permission: "system:role:delete", Handler: deleteRoleHandler},
		{Method: http.MethodGet, Path: "/api/system/roles/:id/grants", Permission: "system:role:query", Handler: getRoleGrantsHandler},
		{Method: http.MethodPut, Path: "/api/system/roles/:id/grants", Permission: "system:role:grant", Handler: saveRoleGrantsHandler},

		// ---- menus ----
		{Method: http.MethodGet, Path: "/api/system/menus", Permission: "system:menu:query", Handler: listMenuResourcesHandler},
		{Method: http.MethodPost, Path: "/api/system/menus", Permission: "system:menu:manage", Handler: createMenuHandler},
		{Method: http.MethodPut, Path: "/api/system/menus/:id", Permission: "system:menu:manage", Handler: updateMenuHandler},
		{Method: http.MethodPut, Path: "/api/system/menus/:id/status", Permission: "system:menu:manage", Handler: updateMenuStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/menus/:id", Permission: "system:menu:manage", Handler: deleteMenuHandler},

		// ---- translations (preset-data localization) ----
		{Method: http.MethodGet, Path: "/api/system/translations", Permission: "system:translation:query", Handler: listTranslationsHandler},
		{Method: http.MethodGet, Path: "/api/system/translations/candidates", Permission: "system:translation:query", Handler: listUntranslatedEntitiesHandler},
		{Method: http.MethodPost, Path: "/api/system/translations", Permission: "system:translation:manage", Handler: createTranslationHandler},
		{Method: http.MethodPut, Path: "/api/system/translations/:id", Permission: "system:translation:manage", Handler: updateTranslationHandler},
		{Method: http.MethodDelete, Path: "/api/system/translations/:id", Permission: "system:translation:manage", Handler: deleteTranslationHandler},
		// Machine translation for the target column: the page calls it while the
		// operator types. It is declared after the CRUD routes on purpose — the
		// permission button for system:translation:manage takes its method and
		// path from the first route that claims the code, and "POST
		// /api/system/translations" is what that button stands for.
		{Method: http.MethodPost, Path: "/api/system/translations/translate", Permission: "system:translation:manage", Handler: translateTextHandler},

		// ---- configs / providers / storage ----
		{Method: http.MethodGet, Path: "/api/system/configs", Permission: "system:config:query", Handler: listConfigsHandler},
		{Method: http.MethodPut, Path: "/api/system/configs/:key", Permission: "system:config:update", Handler: updateConfigHandler},
		{Method: http.MethodGet, Path: "/api/system/auth-providers", Permission: "system:config:auth-provider", Handler: listAuthProvidersHandler},
		{Method: http.MethodPost, Path: "/api/system/auth-providers", Permission: "system:config:auth-provider", Handler: createAuthProviderHandler},
		{Method: http.MethodPut, Path: "/api/system/auth-providers/:id", Permission: "system:config:auth-provider", Handler: updateAuthProviderHandler},
		{Method: http.MethodPut, Path: "/api/system/auth-providers/:id/status", Permission: "system:config:auth-provider", Handler: updateAuthProviderStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/auth-providers/:id", Permission: "system:config:auth-provider", Handler: deleteAuthProviderHandler},
		{Method: http.MethodGet, Path: "/api/system/storage-configs", Permission: "system:config:storage", Handler: listStorageConfigsHandler},
		{Method: http.MethodPost, Path: "/api/system/storage-configs", Permission: "system:config:storage", Handler: createStorageConfigHandler},
		{Method: http.MethodPut, Path: "/api/system/storage-configs/:id", Permission: "system:config:storage", Handler: updateStorageConfigHandler},
		{Method: http.MethodDelete, Path: "/api/system/storage-configs/:id", Permission: "system:config:storage", Handler: deleteStorageConfigHandler},
		{Method: http.MethodPost, Path: "/api/system/storage-configs/:id/test", Permission: "system:config:storage", Handler: testStorageConfigHandler},

		// ---- dictionaries ----
		{Method: http.MethodGet, Path: "/api/system/dictionaries", Permission: "system:dict:query", Handler: listDictionaryTypesHandler},
		{Method: http.MethodPost, Path: "/api/system/dictionaries", AnyOf: []string{"system:dict:create", "system:dict:manage"}, Handler: createDictionaryTypeHandler},
		{Method: http.MethodPut, Path: "/api/system/dictionaries/:id", AnyOf: []string{"system:dict:update", "system:dict:manage"}, Handler: updateDictionaryTypeHandler},
		{Method: http.MethodDelete, Path: "/api/system/dictionaries/:id", AnyOf: []string{"system:dict:delete", "system:dict:manage"}, Handler: deleteDictionaryTypeHandler},
		{Method: http.MethodGet, Path: "/api/system/dictionaries/:id/items", Permission: "system:dict:query", Handler: listDictionaryItemsHandler},
		{Method: http.MethodPost, Path: "/api/system/dictionaries/:id/items", AnyOf: []string{"system:dict:item:create", "system:dict:manage"}, Handler: createDictionaryItemHandler},
		{Method: http.MethodPut, Path: "/api/system/dictionary-items/:id", AnyOf: []string{"system:dict:item:update", "system:dict:manage"}, Handler: updateDictionaryItemHandler},
		{Method: http.MethodDelete, Path: "/api/system/dictionary-items/:id", AnyOf: []string{"system:dict:item:delete", "system:dict:manage"}, Handler: deleteDictionaryItemHandler},
		{Method: http.MethodGet, Path: "/api/system/dictionary-values/:code", Handler: dictionaryItemsByCodeHandler},

		// ---- notices ----
		{Method: http.MethodGet, Path: "/api/system/notices", Permission: "system:notice:query", Handler: listNoticesHandler},
		{Method: http.MethodGet, Path: "/api/system/notices/:id", Permission: "system:notice:query", Handler: getNoticeHandler},
		{Method: http.MethodPost, Path: "/api/system/notices", Permission: "system:notice:manage", Handler: createNoticeHandler},
		{Method: http.MethodPut, Path: "/api/system/notices/:id", Permission: "system:notice:manage", Handler: updateNoticeHandler},
		{Method: http.MethodPost, Path: "/api/system/notices/:id/publish", Permission: "system:notice:manage", FeatureKey: "feature.notice.enabled", Handler: publishNoticeHandler},
		{Method: http.MethodPost, Path: "/api/system/notices/:id/revoke", Permission: "system:notice:manage", Handler: revokeNoticeHandler},
		{Method: http.MethodDelete, Path: "/api/system/notices/:id", Permission: "system:notice:manage", Handler: deleteNoticeHandler},
		{Method: http.MethodGet, Path: "/api/system/my-notices", FeatureKey: "feature.notice.enabled", Handler: listMyNoticesHandler},
		{Method: http.MethodGet, Path: "/api/system/my-notices/:id", FeatureKey: "feature.notice.enabled", Handler: getMyNoticeHandler},
		{Method: http.MethodPost, Path: "/api/system/my-notices/:id/read", FeatureKey: "feature.notice.enabled", Handler: markNoticeReadHandler},

		// ---- files ----
		{Method: http.MethodGet, Path: "/api/system/files", Permission: "system:file:query", Handler: listFilesHandler},
		{Method: http.MethodGet, Path: "/api/system/files/:id", Permission: "system:file:query", Handler: getFileHandler},
		{Method: http.MethodPost, Path: "/api/system/files/upload", Permission: "system:file:upload", FeatureKey: "feature.fileUpload.enabled", Handler: uploadFileHandler},
		{Method: http.MethodGet, Path: "/api/system/files/:id/download", Permission: "system:file:query", Handler: downloadFileHandler},
		{Method: http.MethodDelete, Path: "/api/system/files/:id", Permission: "system:file:delete", Handler: recycleFileHandler},
		{Method: http.MethodPost, Path: "/api/system/files/:id/restore", Permission: "system:file:delete", Handler: restoreFileHandler},
		{Method: http.MethodDelete, Path: "/api/system/files/:id/purge", Permission: "system:file:delete", Handler: purgeFileHandler},
		{Method: http.MethodPost, Path: "/api/system/files/:id/references", Permission: "system:file:upload", Handler: addFileReferenceHandler},
		{Method: http.MethodDelete, Path: "/api/system/file-references/:id", Permission: "system:file:delete", Handler: deleteFileReferenceHandler},

		// ---- sessions / api tokens ----
		{Method: http.MethodGet, Path: "/api/system/sessions", Permission: "system:token:manage", Handler: listSessionsHandler},
		{Method: http.MethodPost, Path: "/api/system/sessions/:id/revoke", Permission: "system:token:manage", Handler: revokeSessionHandler},
		{Method: http.MethodGet, Path: "/api/system/tokens", Permission: "system:token:manage", Handler: listAPITokensHandler},
		{Method: http.MethodPost, Path: "/api/system/tokens", Permission: "system:token:manage", FeatureKey: "feature.apiToken.enabled", Handler: createAPITokenHandler},
		{Method: http.MethodPost, Path: "/api/system/tokens/:id/revoke", Permission: "system:token:manage", Handler: revokeAPITokenHandler},

		// ---- logs ----
		{Method: http.MethodGet, Path: "/api/system/logs/login", Permission: "system:login-log:query", Handler: listLoginLogsHandler},
		{Method: http.MethodGet, Path: "/api/system/logs/operation", Permission: "system:operation-log:query", Handler: listOperationLogsHandler},
		{Method: http.MethodGet, Path: "/api/system/logs/error", Permission: "system:error-log:query", Handler: listErrorLogsHandler},
		{Method: http.MethodPost, Path: "/api/system/logs/error/:id/resolve", Permission: "system:error-log:manage", Handler: resolveErrorLogHandler},

		// ---- jobs ----
		{Method: http.MethodGet, Path: "/api/system/jobs", Permission: "system:job:manage", Handler: listJobsHandler},
		{Method: http.MethodPost, Path: "/api/system/jobs", Permission: "system:job:manage", Handler: createJobHandler},
		{Method: http.MethodPut, Path: "/api/system/jobs/:id", Permission: "system:job:manage", Handler: updateJobHandler},
		{Method: http.MethodDelete, Path: "/api/system/jobs/:id", Permission: "system:job:manage", Handler: deleteJobHandler},
		{Method: http.MethodPost, Path: "/api/system/jobs/:id/run", Permission: "system:job:manage", FeatureKey: "feature.scheduler.enabled", Handler: runJobHandler},
		{Method: http.MethodGet, Path: "/api/system/jobs/:id/logs", Permission: "system:job:manage", Handler: listJobLogsHandler},

		// ---- webhooks ----
		{Method: http.MethodGet, Path: "/api/system/webhooks", Permission: "system:webhook:manage", Handler: listWebhooksHandler},
		{Method: http.MethodPost, Path: "/api/system/webhooks", Permission: "system:webhook:manage", Handler: createWebhookHandler},
		{Method: http.MethodPut, Path: "/api/system/webhooks/:id", Permission: "system:webhook:manage", Handler: updateWebhookHandler},
		{Method: http.MethodPut, Path: "/api/system/webhooks/:id/status", Permission: "system:webhook:manage", Handler: updateWebhookStatusHandler},
		{Method: http.MethodDelete, Path: "/api/system/webhooks/:id", Permission: "system:webhook:manage", Handler: deleteWebhookHandler},
		{Method: http.MethodPost, Path: "/api/system/webhooks/:id/test", Permission: "system:webhook:manage", Handler: testWebhookHandler},

		// ---- cache ----
		{Method: http.MethodGet, Path: "/api/system/cache", Permission: "system:cache:manage", Handler: listCacheKeysHandler},
		{Method: http.MethodDelete, Path: "/api/system/cache/:key", Permission: "system:cache:manage", Handler: deleteCacheKeyHandler},
		{Method: http.MethodPost, Path: "/api/system/cache/clear", Permission: "system:cache:manage", Handler: clearCacheHandler},

		// ---- plugin center ----
		// Appended at the end on purpose: routes.golden stays append-only.
		{Method: http.MethodGet, Path: "/api/system/plugins", Permission: "system:plugin:query", Handler: listPluginsHandler},
		{Method: http.MethodPut, Path: "/api/system/plugins/:id/status", Permission: "system:plugin:manage", Handler: updatePluginStatusHandler},
		// Destructive: drops the plugin's tables. Guarded by the same manage
		// code, and the handler refuses a plugin that is still enabled.
		{Method: http.MethodDelete, Path: "/api/system/plugins/:id", Permission: "system:plugin:manage", Handler: uninstallPluginHandler},
	}
}

// healthHandlerAdapter adapts the zero-dependency health endpoint (it does
// not need app.Context) to the plugin.Route.Handler signature.
func healthHandlerAdapter(*app.Context) http.HandlerFunc { return healthHandler() }

// Hooks provides the middleware implementations the kernel wires into the
// two request chains. Chain order lives in kernel/router.go.
func Hooks(ctx *app.Context) plugin.Hooks {
	return plugin.Hooks{
		Localize:     localizeHook(ctx),
		Authenticate: func(next http.HandlerFunc) http.HandlerFunc { return authenticate(ctx, next) },
		Audit:        func(route plugin.Route, next http.HandlerFunc) http.HandlerFunc { return auditRoute(ctx, route, next) },
		Guard:        func(route plugin.Route, next http.HandlerFunc) http.HandlerFunc { return guard(ctx, route, next) },
		Gate:         func(pluginID string, next http.HandlerFunc) http.HandlerFunc { return pluginGate(ctx, pluginID, next) },
	}
}

// guard reproduces the current nesting exactly: permission OUTER, feature
// INNER (requirePermission(ctx, code, requireFeature(ctx, key, handler))).
func guard(ctx *app.Context, route plugin.Route, next http.HandlerFunc) http.HandlerFunc {
	if route.FeatureKey != "" {
		next = requireFeature(ctx, route.FeatureKey, next)
	}
	switch {
	case len(route.AnyOf) > 0:
		return requireAnyPermission(ctx, route.AnyOf, next)
	case route.Permission != "":
		return requirePermission(ctx, route.Permission, next)
	default:
		return next
	}
}
