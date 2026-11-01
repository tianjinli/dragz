package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/internal/pluginstate"
)

// pluginGate answers 404 for every route owned by a disabled plugin.
//
// It runs outside authentication (see kernel.wrapRoute) so anonymous and
// authenticated callers get exactly the same response as for a path that never
// existed — a disabled plugin must not be discoverable.
func pluginGate(ctx *app.Context, pluginID string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Plugins != nil && !ctx.Plugins.Enabled(pluginID) {
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.not_found")
			return
		}
		next(w, r)
	}
}

// NotFoundHandler renders the same envelope as pluginGate for paths that match
// no route at all, so the two are indistinguishable.
func NotFoundHandler(ctx *app.Context) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.not_found")
	})
	return localizeHook(ctx)(handler)
}

func listPluginsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Plugins == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.plugin.list_failed")
			return
		}
		items, err := ctx.Plugins.List(r.Context())
		if err != nil {
			logx.WithContext(r.Context()).Errorf("list plugins: %v", err)
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.plugin.list_failed")
			return
		}
		for i := range items {
			items[i].Name = localizePluginMeta(r, items[i].ID, items[i].Name, true)
			items[i].Description = localizePluginMeta(r, items[i].ID, items[i].Description, false)
			items[i].LastError = localizedStored(r, items[i].LastError)
		}
		ok(w, r, items)
	}
}

// uninstallPluginHandler drops a disabled plugin's tables, menus and catalog
// row. The service refuses an enabled plugin, so the destructive path is always
// two deliberate steps: disable, then uninstall.
func uninstallPluginHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Plugins == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}

		pluginID := strings.TrimSpace(pathvarValue(r, "id"))
		if !plugin.ValidID(pluginID) {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.plugin.invalid_id")
			return
		}

		err := ctx.Plugins.Uninstall(r.Context(), pluginID)
		switch {
		case err == nil:
			// The plugin's permission codes just left sys_menu; they must leave
			// the enforcer too, otherwise a role keeps a grant that no longer
			// maps to anything.
			if reloadErr := ctx.ReloadAuthorization(r.Context()); reloadErr != nil {
				logx.WithContext(r.Context()).Errorf("reload authorization after uninstalling plugin %q: %v", pluginID, reloadErr)
			}
			ok(w, r, map[string]bool{"success": true})
		case errors.Is(err, pluginstate.ErrNotFound):
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.plugin.not_found")
		case errors.Is(err, pluginstate.ErrBuiltin):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.builtin_protected")
		case errors.Is(err, pluginstate.ErrPluginEnabled):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.enabled_cannot_uninstall")
		case errors.Is(err, pluginstate.ErrDependencyInUse):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.dependency_in_use")
		case errors.Is(err, pluginstate.ErrMigrationBusy):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.migration_busy")
		default:
			logx.WithContext(r.Context()).Errorf("uninstall plugin %q: %v", pluginID, err)
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.plugin.uninstall_failed")
		}
	}
}

// updatePluginStatusHandler switches a plugin on or off.
func updatePluginStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ctx.Plugins == nil {
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
			return
		}

		pluginID := strings.TrimSpace(pathvarValue(r, "id"))
		if !plugin.ValidID(pluginID) {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.plugin.invalid_id")
			return
		}

		var request updateStatusRequest
		if !decode(w, r, &request) {
			return
		}
		if !validStatus(request.Status) {
			fail(w, r, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation")
			return
		}

		var err error
		if request.Status == 1 {
			err = ctx.Plugins.Enable(r.Context(), pluginID)
		} else {
			err = ctx.Plugins.Disable(r.Context(), pluginID)
		}

		switch {
		case err == nil:
			// Both directions change the effective policy set: enabling may
			// have inserted permission buttons and grants, disabling must
			// withdraw that plugin's codes from the enforcer immediately.
			// Best effort: the toggle already committed, and the next reload or
			// restart fixes a transient failure.
			if reloadErr := ctx.ReloadAuthorization(r.Context()); reloadErr != nil {
				logx.WithContext(r.Context()).Errorf("reload authorization after toggling plugin %q: %v", pluginID, reloadErr)
			}
			ok(w, r, map[string]bool{"success": true})
		case errors.Is(err, pluginstate.ErrNotFound):
			fail(w, r, http.StatusNotFound, apierrors.CodeNotFound, "errors.plugin.not_found")
		case errors.Is(err, pluginstate.ErrBuiltin):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.builtin_protected")
		case errors.Is(err, pluginstate.ErrDependencyDisabled):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.dependency_disabled")
		case errors.Is(err, pluginstate.ErrDependencyInUse):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.dependency_in_use")
		case errors.Is(err, pluginstate.ErrMigrationBusy):
			fail(w, r, http.StatusConflict, apierrors.CodeConflict, "errors.plugin.migration_busy")
		case errors.Is(err, pluginstate.ErrMigrationFailed):
			logx.WithContext(r.Context()).Errorf("plugin %q migration failed: %v", pluginID, err)
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.plugin.migration_failed")
		default:
			logx.WithContext(r.Context()).Errorf("toggle plugin %q: %v", pluginID, err)
			fail(w, r, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal")
		}
	}
}
