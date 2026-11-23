package kernel

import (
	"net/http"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/plugin"
)

// Register wires all routes from the registry onto the go-zero server.
//
//	admin : Localize → HostTenant → Gate → Authenticate → Tenant → RateLimit → Audit → Guard → handler
//	public: Localize → HostTenant → Gate → RateLimit → handler
//
// Localize is outermost so every response — including the ones produced by
// middleware below it and by Gate — is translated. HostTenant is next so the
// login page and authenticated calls share one host→tenant result. Gate is
// outside Authenticate so a disabled plugin answers 404 before authentication
// runs: an anonymous caller must not be able to tell "plugin disabled" from
// "route never existed" by receiving a 401.
func Register(server *rest.Server, ctx *app.Context, registry *Registry, hooks plugin.Hooks) {
	public := make([]rest.Route, 0, 16)
	protected := make([]rest.Route, 0, 128)

	for _, bound := range registry.Routes() {
		entry := rest.Route{
			Method:  bound.Route.Method,
			Path:    bound.Route.Path,
			Handler: wrapRoute(ctx, bound, hooks),
		}
		if bound.Route.Public {
			public = append(public, entry)
			continue
		}
		protected = append(protected, entry)
	}

	if len(public) > 0 {
		server.AddRoutes(public)
	}
	if len(protected) > 0 {
		server.AddRoutes(protected)
	}
}

// wrapRoute builds the handler chain for one declared route. Extracted from
// Register so the ordering can be asserted without starting a server.
func wrapRoute(ctx *app.Context, bound Bound, hooks plugin.Hooks) http.HandlerFunc {
	route := bound.Route
	handler := observe(bound.PluginID, route.Handler(ctx))
	if !route.Public {
		if hooks.Guard != nil {
			handler = hooks.Guard(route, handler)
		}
		if hooks.Audit != nil {
			handler = hooks.Audit(route, handler)
		}
		if hooks.RateLimit != nil {
			handler = hooks.RateLimit(route, handler)
		}
		if hooks.Tenant != nil {
			handler = hooks.Tenant(handler)
		}
		if hooks.Authenticate != nil {
			handler = hooks.Authenticate(handler)
		}
	} else if hooks.RateLimit != nil {
		handler = hooks.RateLimit(route, handler)
	}
	if hooks.Gate != nil {
		handler = hooks.Gate(bound.PluginID, handler)
	}
	if hooks.HostTenant != nil {
		handler = hooks.HostTenant(handler)
	}
	if hooks.Localize != nil {
		handler = hooks.Localize(handler)
	}
	// Metering wraps the whole chain, so a request rejected by Gate, HostTenant
	// or the limiter is counted too: "everything is 401" is exactly the kind of
	// picture an operator needs. A nil registry makes this a no-op.
	return ctx.Metrics.Middleware(route.Path, handler)
}

// observe tags every log line emitted while serving the route with the
// owning plugin ID.
func observe(pluginID string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next(w, r.WithContext(logx.ContextWithFields(r.Context(), logx.Field("plugin", pluginID))))
	}
}
