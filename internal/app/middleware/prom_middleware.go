package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type PrometheusMiddleware struct {
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
}

// NewPrometheusMiddleware creates a new instance of PrometheusMiddleware and registers the metrics.
// middle := NewPrometheusMiddleware()
// rootGroup.Use(middle.HandlePrometheus())
// rootGroup.GET("/metrics", gin.WrapH(promhttp.Handler()))
func NewPrometheusMiddleware() *PrometheusMiddleware {
	middleware := &PrometheusMiddleware{
		httpRequestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		httpRequestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path", "status"},
		),
	}
	prometheus.MustRegister(middleware.httpRequestsTotal)
	prometheus.MustRegister(middleware.httpRequestDuration)
	return middleware
}

func (m *PrometheusMiddleware) HandlePrometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		method := c.Request.Method
		// Use `FullPath` to retrieve the route template (e.g., `/user/:id`) rather than the concrete path.
		path := c.FullPath()
		status := strconv.Itoa(c.Writer.Status())

		m.httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		m.httpRequestDuration.WithLabelValues(method, path, status).Observe(duration.Seconds())
	}
}
