// Package metrics is the process-local request meter.
//
// It exists so a self-hosted deployment can answer "is this slow, and where"
// without pulling in a metrics SDK: one counter and one duration summary per
// (method, route, status), rendered in the Prometheus text format that every
// scraper already understands.
//
// Route templates are used as the label, never the raw path, so a request to
// /api/system/users/42 and /api/system/users/43 land in the same series. Raw
// paths would create one series per id and blow up memory.
package metrics

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Registry collects request counters. The zero value is not usable; call New.
type Registry struct {
	mu       sync.Mutex
	started  time.Time
	inFlight int64
	series   map[seriesKey]*series
}

type seriesKey struct {
	method string
	route  string
	status int
}

type series struct {
	count       uint64
	durationSum float64
	durationMax float64
}

// New returns an empty registry stamped with the process start time.
func New() *Registry {
	return &Registry{started: time.Now(), series: make(map[seriesKey]*series)}
}

// Observe records one finished request. duration is wall time spent inside the
// handler chain; status is the code actually written (0 is reported as 200,
// because a handler that writes nothing is a success in net/http).
func (r *Registry) Observe(method, route string, status int, duration time.Duration) {
	if r == nil {
		return
	}
	if status == 0 {
		status = http.StatusOK
	}
	key := seriesKey{method: method, route: route, status: status}
	seconds := duration.Seconds()

	r.mu.Lock()
	item, found := r.series[key]
	if !found {
		item = &series{}
		r.series[key] = item
	}
	item.count++
	item.durationSum += seconds
	if seconds > item.durationMax {
		item.durationMax = seconds
	}
	r.mu.Unlock()
}

// InFlight adjusts the gauge of requests currently being served.
func (r *Registry) InFlight(delta int64) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.inFlight += delta
	r.mu.Unlock()
}

// WriteTo renders the Prometheus text exposition format (version 0.0.4).
//
// Series are sorted so two scrapes of an unchanged process produce identical
// bytes: an unsorted map would make every diff look like traffic.
func (r *Registry) WriteTo(w io.Writer) (int64, error) {
	r.mu.Lock()
	keys := make([]seriesKey, 0, len(r.series))
	snapshot := make(map[seriesKey]series, len(r.series))
	for key, item := range r.series {
		keys = append(keys, key)
		snapshot[key] = *item
	}
	inFlight := r.inFlight
	started := r.started
	r.mu.Unlock()

	sort.Slice(keys, func(i, j int) bool {
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].status < keys[j].status
	})

	var builder strings.Builder
	builder.WriteString("# HELP dragz_http_requests_total Requests served, by route template and status.\n")
	builder.WriteString("# TYPE dragz_http_requests_total counter\n")
	for _, key := range keys {
		builder.WriteString("dragz_http_requests_total")
		builder.WriteString(labels(key))
		builder.WriteString(" ")
		builder.WriteString(strconv.FormatUint(snapshot[key].count, 10))
		builder.WriteString("\n")
	}

	builder.WriteString("# HELP dragz_http_request_duration_seconds Time spent serving a request.\n")
	builder.WriteString("# TYPE dragz_http_request_duration_seconds summary\n")
	for _, key := range keys {
		builder.WriteString("dragz_http_request_duration_seconds_sum")
		builder.WriteString(labels(key))
		builder.WriteString(" ")
		builder.WriteString(strconv.FormatFloat(snapshot[key].durationSum, 'f', 6, 64))
		builder.WriteString("\n")
		builder.WriteString("dragz_http_request_duration_seconds_count")
		builder.WriteString(labels(key))
		builder.WriteString(" ")
		builder.WriteString(strconv.FormatUint(snapshot[key].count, 10))
		builder.WriteString("\n")
	}

	builder.WriteString("# HELP dragz_http_requests_in_flight Requests being served right now.\n")
	builder.WriteString("# TYPE dragz_http_requests_in_flight gauge\n")
	builder.WriteString("dragz_http_requests_in_flight ")
	builder.WriteString(strconv.FormatInt(inFlight, 10))
	builder.WriteString("\n")
	builder.WriteString("# HELP dragz_process_start_time_seconds Unix time the process started.\n")
	builder.WriteString("# TYPE dragz_process_start_time_seconds gauge\n")
	builder.WriteString("dragz_process_start_time_seconds ")
	builder.WriteString(strconv.FormatFloat(float64(started.UnixNano())/1e9, 'f', 3, 64))
	builder.WriteString("\n")

	text := builder.String()
	written, err := io.WriteString(w, text)
	return int64(written), err
}

// Handler serves the registry as text/plain, the format a Prometheus scrape
// expects. A nil registry reports 404 so a deployment that did not enable the
// meter does not advertise an endpoint it cannot serve.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if r == nil {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = r.WriteTo(w)
	}
}

// statusRecorder remembers the code a handler wrote so the middleware can label
// the series correctly. net/http gives no way to read it back.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(data []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(data)
}

// Unwrap lets http.ResponseController reach the underlying writer for flushing
// and hijacking (WebSocket upgrade).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Hijack forwards the connection upgrade a WebSocket handler needs. Without it
// the type assertion in the upgrader fails and /api/ws stops working the moment
// the meter is switched on.
func (s *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("metrics: underlying writer does not support hijacking")
	}
	return hijacker.Hijack()
}

// Flush forwards streaming flushes (server-sent events).
func (s *statusRecorder) Flush() {
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Middleware times one route. route is the declared template, not r.URL.Path.
func (r *Registry) Middleware(route string, next http.HandlerFunc) http.HandlerFunc {
	if r == nil {
		return next
	}
	return func(w http.ResponseWriter, req *http.Request) {
		r.InFlight(1)
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		defer func() {
			r.InFlight(-1)
			status := recorder.status
			if status == 0 {
				status = http.StatusOK
			}
			r.Observe(req.Method, route, status, time.Since(started))
		}()
		next(recorder, req)
	}
}

func labels(key seriesKey) string {
	return fmt.Sprintf("{method=%q,route=%q,status=%q}",
		key.method, key.route, strconv.Itoa(key.status))
}
