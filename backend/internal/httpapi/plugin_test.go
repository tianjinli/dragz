package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/rest/pathvar"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pluginstate"
	"dragz-admin/backend/internal/store"
)

// fakeRuntime is a pluginstate.Runtime with observable calls.
type fakeRuntime struct {
	enabled        map[string]bool
	builtin        map[string]bool
	infos          []pluginstate.Info
	listErr        error
	enableErr      error
	disableErr     error
	enableCalls    []string
	disableCall    []string
	uninstallErr   error
	uninstallCalls []string
}

func (f *fakeRuntime) Enabled(id string) bool { return f.enabled[id] }
func (f *fakeRuntime) Builtin(id string) bool { return f.builtin[id] }

func (f *fakeRuntime) List(context.Context) ([]pluginstate.Info, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.infos, nil
}

func (f *fakeRuntime) Enable(_ context.Context, id string) error {
	f.enableCalls = append(f.enableCalls, id)
	return f.enableErr
}

func (f *fakeRuntime) Disable(_ context.Context, id string) error {
	f.disableCall = append(f.disableCall, id)
	return f.disableErr
}

func (f *fakeRuntime) Uninstall(_ context.Context, id string) error {
	f.uninstallCalls = append(f.uninstallCalls, id)
	return f.uninstallErr
}

func newRuntime() *fakeRuntime {
	return &fakeRuntime{enabled: map[string]bool{}, builtin: map[string]bool{}}
}

// newTestContext returns a context wired to a sqlmock database. With no query
// expectations the Casbin reload fails, which is exactly what the handler must
// tolerate: the plugin is already enabled, so a failed reload is only logged.
func newTestContext(t *testing.T, runtime pluginstate.Runtime) *app.Context {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	mock.ExpectClose()
	t.Cleanup(func() { _ = db.Close() })
	return &app.Context{Auth: store.NewAuthRepository(&store.DB{DB: db}), Plugins: runtime}
}

func statusRequest(t *testing.T, pluginID string, status int8) *http.Request {
	t.Helper()
	body := strings.NewReader(fmt.Sprintf(`{"status":%d}`, status))
	// The URL is fixed: the plugin ID reaches the handler through pathvar, and
	// an invalid ID must not be able to break URL parsing.
	request := httptest.NewRequest(http.MethodPut, "/api/system/plugins/x/status", body)
	request.Header.Set("Content-Type", "application/json")
	return pathvar.WithVars(request, map[string]string{"id": pluginID})
}

func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) envelope {
	t.Helper()
	var body envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestListPluginsHandlerReturnsItems(t *testing.T) {
	runtime := newRuntime()
	runtime.infos = []pluginstate.Info{
		{ID: "system", Name: "系统管理", Builtin: true, Enabled: true, DependsOn: []string{}},
		{ID: "demo", Name: "示例插件", Enabled: false, DependsOn: []string{"system"}, PendingMigrations: 1},
	}
	ctx := newTestContext(t, runtime)

	rec := httptest.NewRecorder()
	listPluginsHandler(ctx)(rec, httptest.NewRequest(http.MethodGet, "/api/system/plugins", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var payload struct {
		Data []pluginstate.Info `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Data) != 2 || payload.Data[1].PendingMigrations != 1 {
		t.Fatalf("payload = %+v, want two plugins", payload.Data)
	}
}

func TestListPluginsHandlerWithoutRuntime(t *testing.T) {
	rec := httptest.NewRecorder()
	listPluginsHandler(&app.Context{})(rec, httptest.NewRequest(http.MethodGet, "/api/system/plugins", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// The reload after a successful enable is best effort: it must not turn a
// successful enable into an error response.
func TestUpdatePluginStatusEnables(t *testing.T) {
	runtime := newRuntime()
	ctx := newTestContext(t, runtime)

	rec := httptest.NewRecorder()
	updatePluginStatusHandler(ctx)(rec, statusRequest(t, "demo", 1))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if len(runtime.enableCalls) != 1 || runtime.enableCalls[0] != "demo" {
		t.Fatalf("enable calls = %v, want [demo]", runtime.enableCalls)
	}
	if len(runtime.disableCall) != 0 {
		t.Fatalf("disable calls = %v, want none", runtime.disableCall)
	}
}

func TestUpdatePluginStatusDisables(t *testing.T) {
	runtime := newRuntime()
	ctx := newTestContext(t, runtime)

	rec := httptest.NewRecorder()
	updatePluginStatusHandler(ctx)(rec, statusRequest(t, "demo", 0))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(runtime.disableCall) != 1 || runtime.disableCall[0] != "demo" {
		t.Fatalf("disable calls = %v, want [demo]", runtime.disableCall)
	}
}

func TestUpdatePluginStatusMapsSentinelErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int
	}{
		{"not found", pluginstate.ErrNotFound, http.StatusNotFound, 40401},
		{"builtin", pluginstate.ErrBuiltin, http.StatusConflict, 40902},
		{"dependency disabled", pluginstate.ErrDependencyDisabled, http.StatusConflict, 40902},
		{"dependency in use", pluginstate.ErrDependencyInUse, http.StatusConflict, 40902},
		{"busy", pluginstate.ErrMigrationBusy, http.StatusConflict, 40902},
		{"migration failed", pluginstate.ErrMigrationFailed, http.StatusInternalServerError, 50000},
		{"unexpected", errors.New("kaboom"), http.StatusInternalServerError, 50000},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runtime := newRuntime()
			runtime.enableErr = testCase.err
			ctx := newTestContext(t, runtime)

			rec := httptest.NewRecorder()
			updatePluginStatusHandler(ctx)(rec, statusRequest(t, "demo", 1))

			if rec.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, testCase.wantStatus)
			}
			if body := decodeEnvelope(t, rec); body.Code != testCase.wantCode {
				t.Fatalf("code = %d, want %d", body.Code, testCase.wantCode)
			}
		})
	}
}

func TestUpdatePluginStatusRejectsInvalidID(t *testing.T) {
	runtime := newRuntime()
	ctx := newTestContext(t, runtime)

	rec := httptest.NewRecorder()
	updatePluginStatusHandler(ctx)(rec, statusRequest(t, "BAD ID", 1))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(runtime.enableCalls) != 0 {
		t.Fatalf("enable must not be called for an invalid ID")
	}
}

// uninstallRequest targets DELETE /api/system/plugins/:id; the ID reaches the
// handler through pathvar so an invalid one cannot break URL parsing.
func uninstallRequest(t *testing.T, pluginID string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodDelete, "/api/system/plugins/x", nil)
	return pathvar.WithVars(request, map[string]string{"id": pluginID})
}

func TestUninstallPluginHandlerSucceeds(t *testing.T) {
	runtime := newRuntime()
	ctx := newTestContext(t, runtime)

	rec := httptest.NewRecorder()
	uninstallPluginHandler(ctx)(rec, uninstallRequest(t, "demo"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if len(runtime.uninstallCalls) != 1 || runtime.uninstallCalls[0] != "demo" {
		t.Fatalf("uninstall calls = %v, want [demo]", runtime.uninstallCalls)
	}
}

func TestUninstallPluginHandlerMapsSentinelErrors(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   int
	}{
		{"not found", pluginstate.ErrNotFound, http.StatusNotFound, 40401},
		{"builtin", pluginstate.ErrBuiltin, http.StatusConflict, 40902},
		{"still enabled", pluginstate.ErrPluginEnabled, http.StatusConflict, 40902},
		{"dependency in use", pluginstate.ErrDependencyInUse, http.StatusConflict, 40902},
		{"busy", pluginstate.ErrMigrationBusy, http.StatusConflict, 40902},
		{"unexpected", errors.New("kaboom"), http.StatusInternalServerError, 50000},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runtime := newRuntime()
			runtime.uninstallErr = testCase.err
			ctx := newTestContext(t, runtime)

			rec := httptest.NewRecorder()
			uninstallPluginHandler(ctx)(rec, uninstallRequest(t, "demo"))

			if rec.Code != testCase.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, testCase.wantStatus)
			}
			if body := decodeEnvelope(t, rec); body.Code != testCase.wantCode {
				t.Fatalf("code = %d, want %d", body.Code, testCase.wantCode)
			}
		})
	}
}

func TestUninstallPluginHandlerRejectsInvalidID(t *testing.T) {
	runtime := newRuntime()
	ctx := newTestContext(t, runtime)

	rec := httptest.NewRecorder()
	uninstallPluginHandler(ctx)(rec, uninstallRequest(t, "BAD ID"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if len(runtime.uninstallCalls) != 0 {
		t.Fatal("uninstall must not be called for an invalid ID")
	}
}

func TestPluginGateBlocksDisabledPlugin(t *testing.T) {
	runtime := newRuntime()
	runtime.enabled["demo"] = false
	ctx := &app.Context{Plugins: runtime}

	called := false
	rec := httptest.NewRecorder()
	pluginGate(ctx, "demo", func(http.ResponseWriter, *http.Request) { called = true })(rec, httptest.NewRequest(http.MethodGet, "/api/demo/ping", nil))

	if called {
		t.Fatal("handler must not run for a disabled plugin")
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if body := decodeEnvelope(t, rec); body.Code != 40401 {
		t.Fatalf("code = %d, want 40401", body.Code)
	}
}

func TestPluginGateAllowsEnabledPlugin(t *testing.T) {
	runtime := newRuntime()
	runtime.enabled["demo"] = true
	ctx := &app.Context{Plugins: runtime}

	called := false
	rec := httptest.NewRecorder()
	pluginGate(ctx, "demo", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(rec, httptest.NewRequest(http.MethodGet, "/api/demo/ping", nil))

	if !called || rec.Code != http.StatusOK {
		t.Fatalf("handler must run for an enabled plugin (called=%v status=%d)", called, rec.Code)
	}
}

// A nil runtime means "everything enabled", so a partially initialised binary
// does not 404 the whole API.
func TestPluginGateWithoutRuntimeIsOpen(t *testing.T) {
	called := false
	rec := httptest.NewRecorder()
	pluginGate(&app.Context{}, "demo", func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})(rec, httptest.NewRequest(http.MethodGet, "/api/demo/ping", nil))

	if !called {
		t.Fatal("gate must be open when no runtime is installed")
	}
}

// The two 404 sources must be byte-identical, otherwise the response tells an
// attacker whether a plugin exists but is switched off.
func TestNotFoundHandlerMatchesGateResponse(t *testing.T) {
	runtime := newRuntime()
	ctx := &app.Context{Plugins: runtime}

	gated := httptest.NewRecorder()
	pluginGate(ctx, "demo", func(http.ResponseWriter, *http.Request) {})(gated, httptest.NewRequest(http.MethodGet, "/api/demo/ping", nil))

	unknown := httptest.NewRecorder()
	NotFoundHandler(ctx).ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil))

	if gated.Code != unknown.Code {
		t.Fatalf("status differs: gated=%d unknown=%d", gated.Code, unknown.Code)
	}
	if gated.Body.String() != unknown.Body.String() {
		t.Fatalf("body differs:\n gated=%s\n unknown=%s", gated.Body.String(), unknown.Body.String())
	}
}
