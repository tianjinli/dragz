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
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/platform/plugincenter"
	"dragz-admin/backend/plugins/all"
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

	plugins, err := plugincenter.New(
		store.NewPluginRepository(ctx.DB),
		registry.Metas(),
		registry.MigrationSources(),
		registry.Routes(),
	)
	logx.Must(err)
	logx.Must(plugins.RunStartup(context.Background()))
	ctx.Plugins = plugins

	// Every replica keeps its own snapshot, so a toggle served by one replica
	// has to become visible on the others without a restart. The watcher is
	// stopped by app.Context.Close.
	watchCtx, stopWatch := context.WithCancel(context.Background())
	ctx.Defer(stopWatch)
	go plugins.Watch(watchCtx, plugincenter.DefaultWatchInterval)

	// The seed may have inserted permissions and grants; make them effective
	// without requiring a restart.
	logx.Must(ctx.ReloadAuthorization(context.Background()))

	kernel.Register(server, ctx, registry, httpapi.Hooks(ctx))
}

// NotFoundHandler renders the same 404 envelope as a route belonging to a
// disabled plugin, so the two are indistinguishable. Wired in cmd/admin-api via
// rest.WithNotFoundHandler.
func NotFoundHandler(ctx *app.Context) http.Handler {
	return httpapi.NotFoundHandler(ctx)
}
