package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const ns = "graydraft_gateway"

var (
	enabled bool
	httpTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Namespace: ns, Name: "http_requests_total", Help: "HTTP requests"},
		[]string{"method", "route", "code"},
	)
	httpDur = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: ns, Name: "http_request_duration_seconds", Help: "HTTP latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)
)

// Init registers metrics (O-12).
func Init() {
	if enabled {
		return
	}
	prometheus.MustRegister(httpTotal, httpDur)
	enabled = true
}

// Handler returns Prometheus scrape handler.
func Handler() gin.HandlerFunc {
	Init()
	return gin.WrapH(promhttp.Handler())
}

// Middleware records request metrics.
func Middleware() gin.HandlerFunc {
	Init()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		if route == "/metrics" {
			return
		}
		httpTotal.WithLabelValues(c.Request.Method, route, strconv.Itoa(c.Writer.Status())).Inc()
		httpDur.WithLabelValues(c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}
