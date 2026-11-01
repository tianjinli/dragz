package kernel

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/plugin"
)

// traceHooks records the order in which each middleware runs and rejects
// requests for the "disabled" plugin with a 404, mimicking the real gate.
func traceHooks(order *[]string, disabledPlugin string) plugin.Hooks {
	mark := func(name string, next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			*order = append(*order, name)
			next(w, r)
		}
	}
	return plugin.Hooks{
		Localize: func(next http.HandlerFunc) http.HandlerFunc { return mark("localize", next) },
		Authenticate: func(next http.HandlerFunc) http.HandlerFunc {
			return mark("authenticate", next)
		},
		Audit: func(_ plugin.Route, next http.HandlerFunc) http.HandlerFunc {
			return mark("audit", next)
		},
		Guard: func(_ plugin.Route, next http.HandlerFunc) http.HandlerFunc {
			return mark("guard", next)
		},
		Gate: func(pluginID string, next http.HandlerFunc) http.HandlerFunc {
			return func(w http.ResponseWriter, r *http.Request) {
				*order = append(*order, "gate")
				if pluginID == disabledPlugin {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"code":40401,"message":"资源不存在"}`))
					return
				}
				next(w, r)
			}
		},
	}
}

func serve(t *testing.T, bound Bound, hooks plugin.Hooks) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, bound.Route.Path, nil)
	wrapRoute(&app.Context{}, bound, hooks)(rec, req)
	return rec
}

func TestWrapRouteProtectedChainOrder(t *testing.T) {
	var order []string
	bound := Bound{
		PluginID: "demo",
		Route: plugin.Route{
			Method: http.MethodGet,
			Path:   "/api/demo/ping",
			Handler: func(*app.Context) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					order = append(order, "handler")
					w.WriteHeader(http.StatusOK)
				}
			},
		},
	}

	rec := serve(t, bound, traceHooks(&order, ""))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := []string{"localize", "gate", "authenticate", "audit", "guard", "handler"}
	if strings.Join(order, ">") != strings.Join(want, ">") {
		t.Fatalf("chain order = %v, want %v", order, want)
	}
}

func TestWrapRoutePublicSkipsAuthChain(t *testing.T) {
	var order []string
	bound := Bound{
		PluginID: "demo",
		Route: plugin.Route{
			Method: http.MethodGet,
			Path:   "/api/demo/health",
			Public: true,
			Handler: func(*app.Context) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) { order = append(order, "handler") }
			},
		},
	}

	serve(t, bound, traceHooks(&order, ""))

	want := []string{"localize", "gate", "handler"}
	if strings.Join(order, ">") != strings.Join(want, ">") {
		t.Fatalf("chain order = %v, want %v", order, want)
	}
}

// A disabled plugin must answer 404 for protected routes WITHOUT running
// authentication, otherwise an anonymous caller gets 401 and learns that the
// route exists.
func TestWrapRouteDisabledPluginReturns404BeforeAuth(t *testing.T) {
	var order []string
	bound := Bound{
		PluginID: "disabled",
		Route: plugin.Route{
			Method:     http.MethodGet,
			Path:       "/api/disabled/ping",
			Permission: "demo:ping",
			Handler: func(*app.Context) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) { order = append(order, "handler") }
			},
		},
	}

	rec := serve(t, bound, traceHooks(&order, "disabled"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "40401") {
		t.Fatalf("body = %q, want the not-found envelope", rec.Body.String())
	}
	for _, name := range order {
		if name == "authenticate" || name == "guard" || name == "audit" || name == "handler" {
			t.Fatalf("chain ran %q after the gate blocked the request: %v", name, order)
		}
	}
}

// Localize must be the outermost wrapper so the 404 produced by Gate is
// translated like every other response.
func TestWrapRouteLocalizeIsOutermost(t *testing.T) {
	var order []string
	bound := Bound{
		PluginID: "disabled",
		Route: plugin.Route{
			Method:  http.MethodGet,
			Path:    "/api/disabled/public",
			Public:  true,
			Handler: func(*app.Context) http.HandlerFunc { return func(w http.ResponseWriter, _ *http.Request) {} },
		},
	}

	serve(t, bound, traceHooks(&order, "disabled"))

	if len(order) == 0 || order[0] != "localize" {
		t.Fatalf("first wrapper = %v, want localize outermost", order)
	}
}

// The gate must also apply to public routes: a disabled plugin's login or
// health endpoints disappear with the rest of it.
func TestWrapRouteDisabledPluginGatesPublicRoutes(t *testing.T) {
	var order []string
	bound := Bound{
		PluginID: "disabled",
		Route: plugin.Route{
			Method:  http.MethodPost,
			Path:    "/api/disabled/login",
			Public:  true,
			Handler: func(*app.Context) http.HandlerFunc { return func(w http.ResponseWriter, _ *http.Request) {} },
		},
	}

	rec := serve(t, bound, traceHooks(&order, "disabled"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
