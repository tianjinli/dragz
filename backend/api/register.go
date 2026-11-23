// Package api wires the kernel, the plugin center and the HTTP handlers
// into a runnable server.
package api

import (
	"context"
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/httpapi"
	"dragz-admin/backend/internal/kernel"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/platform/notifier"
	"dragz-admin/backend/platform/plugincenter"
	"dragz-admin/backend/plugins/all"
	"dragz-admin/backend/plugins/notify"
)

// RegisterRoutes assembles every compiled-in plugin onto the server.
// Route declarations come from plugins; middleware order lives in the kernel;
// middleware implementations come from the platform layer (httpapi).
//
// Startup order matters: the plugin runtime must be up (schema, seeds, catalog,
// migrations) before the router is built, because the route gate reads its
// snapshot and the Casbin policies may have just been seeded.
func RegisterRoutes(server *rest.Server, ctx *app.Context) {
	registry, err := kernel.Build(all.All())
	logx.Must(err)

	schemaDB := ctx.SchemaDB
	if schemaDB == nil {
		schemaDB = ctx.DB
	}
	plugins, err := plugincenter.New(
		store.NewPluginRepository(schemaDB),
		registry.Metas(),
		registry.MigrationSources(),
		registry.Routes(),
	)
	logx.Must(err)
	// Plugin bootstrap seeds menu grants for the built-in default tenant. A
	// non-zero tenant is explicit: app_fill_tenant_id no longer defaults to 1.
	startupCtx := ctxkey.WithTenantID(ctxkey.WithRLSBypass(context.Background(), true), 1)
	logx.Must(plugins.RunStartup(startupCtx))
	ctx.Plugins = plugins

	// Install the notification provider at boot, not on the first HTTP call:
	// a plugin publishing notify.send during startup must not lose it. The
	// provider checks the live plugin switch on every Send, so subscribing
	// even while notify is disabled is safe and survives runtime toggles.
	wireNotifier(ctx)

	// Every replica keeps its own snapshot, so a toggle served by one replica
	// has to become visible on the others without a restart. The watcher is
	// stopped by app.Context.Close.
	watchCtx, stopWatch := context.WithCancel(context.Background())
	ctx.Defer(stopWatch)
	go plugins.Watch(watchCtx, plugincenter.DefaultWatchInterval)

	// The seed may have inserted permissions and grants; make them effective
	// without requiring a restart.
	logx.Must(ctx.ReloadAuthorization(startupCtx))

	kernel.Register(server, ctx, registry, httpapi.Hooks(ctx))
}

// wireNotifier publishes notify's delivery capability through the shared
// context and registers its eventbus entry point once at startup. This is the
// only place the composition root names the provider plugin; consumers only
// know platform/notifier.
func wireNotifier(ctx *app.Context) {
	provider := notify.NewNotifier(ctx)
	ctx.Notifier = provider
	if ctx.Bus != nil {
		ctx.Bus.Subscribe(notifier.EventName, provider.HandleEvent)
	}
}

// NotFoundHandler renders the same 404 envelope as a route belonging to a
// disabled plugin, so the two are indistinguishable. Wired in cmd/admin-api via
// rest.WithNotFoundHandler.
func NotFoundHandler(ctx *app.Context) http.Handler {
	return httpapi.NotFoundHandler(ctx)
}
