package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dragz-admin/backend/internal/metrics"
)

func TestObserveAggregatesByRouteTemplate(t *testing.T) {
	registry := metrics.New()
	registry.Observe(http.MethodGet, "/api/system/users/:id", http.StatusOK, 10*time.Millisecond)
	registry.Observe(http.MethodGet, "/api/system/users/:id", http.StatusOK, 30*time.Millisecond)
	registry.Observe(http.MethodGet, "/api/system/users/:id", http.StatusInternalServerError, time.Second)

	var builder strings.Builder
	if _, err := registry.WriteTo(&builder); err != nil {
		t.Fatalf("write: %v", err)
	}
	text := builder.String()

	for _, want := range []string{
		`dragz_http_requests_total{method="GET",route="/api/system/users/:id",status="200"} 2`,
		`dragz_http_requests_total{method="GET",route="/api/system/users/:id",status="500"} 1`,
		`dragz_http_request_duration_seconds_sum{method="GET",route="/api/system/users/:id",status="200"} 0.040000`,
		`dragz_http_request_duration_seconds_count{method="GET",route="/api/system/users/:id",status="200"} 2`,
		"dragz_http_requests_in_flight 0",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics output is missing %q\n--- output ---\n%s", want, text)
		}
	}
}

func TestWriteToIsStable(t *testing.T) {
	registry := metrics.New()
	registry.Observe(http.MethodGet, "/api/b", http.StatusOK, time.Millisecond)
	registry.Observe(http.MethodGet, "/api/a", http.StatusOK, time.Millisecond)
	registry.Observe(http.MethodPost, "/api/a", http.StatusCreated, time.Millisecond)

	first := render(t, registry)
	second := render(t, registry)
	if first != second {
		t.Fatalf("two renders of one unchanged registry differ:\n%s\n---\n%s", first, second)
	}
	if strings.Index(first, `route="/api/a"`) > strings.Index(first, `route="/api/b"`) {
		t.Errorf("series are not sorted by route:\n%s", first)
	}
}

func TestMiddlewareRecordsStatusAndDuration(t *testing.T) {
	registry := metrics.New()
	handler := registry.Middleware("/api/widgets/:id", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/widgets/7", nil))

	text := render(t, registry)
	if !strings.Contains(text, `status="404"} 1`) {
		t.Errorf("middleware did not record the written status:\n%s", text)
	}
	if !strings.Contains(text, "dragz_http_requests_in_flight 0") {
		t.Errorf("in-flight gauge was not released:\n%s", text)
	}
}

// A handler that writes a body without calling WriteHeader is a 200; the meter
// must not record it as status 0.
func TestMiddlewareDefaultsToOK(t *testing.T) {
	registry := metrics.New()
	handler := registry.Middleware("/api/ping", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "pong")
	})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/ping", nil))

	if text := render(t, registry); !strings.Contains(text, `status="200"} 1`) {
		t.Errorf("implicit 200 was not recorded:\n%s", text)
	}
}

func TestNilRegistryIsInert(t *testing.T) {
	var registry *metrics.Registry
	registry.Observe(http.MethodGet, "/api/anything", http.StatusOK, time.Millisecond)
	registry.InFlight(1)

	called := false
	handler := registry.Middleware("/api/anything", func(http.ResponseWriter, *http.Request) { called = true })
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/anything", nil))
	if !called {
		t.Fatal("a nil registry must pass the request through untouched")
	}

	// The scrape endpoint has to look unconfigured rather than empty.
	recorder := httptest.NewRecorder()
	registry.Handler()(recorder, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))
	if recorder.Code != http.StatusNotFound {
		t.Errorf("nil registry scrape: got %d, want 404", recorder.Code)
	}
}

func TestHandlerServesPrometheusContentType(t *testing.T) {
	recorder := httptest.NewRecorder()
	metrics.New().Handler()(recorder, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("scrape: got %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Errorf("scrape content type: got %q", got)
	}
}

func render(t *testing.T, registry *metrics.Registry) string {
	t.Helper()
	var builder strings.Builder
	if _, err := registry.WriteTo(&builder); err != nil {
		t.Fatalf("write: %v", err)
	}
	return builder.String()
}
