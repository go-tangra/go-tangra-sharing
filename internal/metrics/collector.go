package metrics

import (
	"context"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tx7do/kratos-bootstrap/bootstrap"

	commonMetrics "github.com/go-tangra/go-tangra-common/metrics"
)

const namespace = "tangra"
const subsystem = "sharing"

// Collector holds all Prometheus metrics for the sharing module.
type Collector struct {
	log    *log.Helper
	server *commonMetrics.MetricsServer

	// SharedLink metrics
	SharedLinksByStatus *prometheus.GaugeVec

	// SharePolicy metrics
	SharePoliciesTotal prometheus.Gauge

	// gRPC request metrics
	RequestDuration *prometheus.HistogramVec
	RequestsTotal   *prometheus.CounterVec
}

// NewCollector creates and registers all sharing Prometheus metrics.
func NewCollector(ctx *bootstrap.Context) *Collector {
	c := &Collector{
		log: ctx.NewLoggerHelper("sharing/metrics"),

		SharedLinksByStatus: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "shared_links_by_status",
			Help:      "Number of shared links by status.",
		}, []string{"status"}),

		SharePoliciesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "share_policies_total",
			Help:      "Total number of share policies.",
		}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "Histogram of gRPC request durations in seconds.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method"}),

		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests by method and status.",
		}, []string{"method", "status"}),
	}

	prometheus.MustRegister(
		c.SharedLinksByStatus,
		c.SharePoliciesTotal,
		c.RequestDuration,
		c.RequestsTotal,
	)

	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = ":9610"
	}
	c.server = commonMetrics.NewMetricsServer(addr, nil, ctx.GetLogger())

	go func() {
		if err := c.server.Start(); err != nil {
			c.log.Errorf("Metrics server failed: %v", err)
		}
	}()

	return c
}

// Stop shuts down the metrics HTTP server.
func (c *Collector) Stop(ctx context.Context) {
	if c.server != nil {
		c.server.Stop(ctx)
	}
}

// Middleware returns a Kratos middleware that records gRPC request metrics.
func (c *Collector) Middleware() middleware.Middleware {
	return commonMetrics.NewServerMiddleware(c.RequestDuration, c.RequestsTotal)
}

// --- SharedLink helpers ---

// SharedLinkCreated increments the shared link counter for the "active" status.
func (c *Collector) SharedLinkCreated() {
	c.SharedLinksByStatus.WithLabelValues("active").Inc()
}

// SharedLinkViewed moves a shared link from "active" to "viewed" status.
func (c *Collector) SharedLinkViewed() {
	c.SharedLinksByStatus.WithLabelValues("active").Dec()
	c.SharedLinksByStatus.WithLabelValues("viewed").Inc()
}

// SharedLinkRevoked moves a shared link to "revoked" status from the given previous status.
func (c *Collector) SharedLinkRevoked(wasViewed bool) {
	if wasViewed {
		c.SharedLinksByStatus.WithLabelValues("viewed").Dec()
	} else {
		c.SharedLinksByStatus.WithLabelValues("active").Dec()
	}
	c.SharedLinksByStatus.WithLabelValues("revoked").Inc()
}

// --- SharePolicy helpers ---

// PolicyCreated increments the share policy counter.
func (c *Collector) PolicyCreated() {
	c.SharePoliciesTotal.Inc()
}

// PolicyDeleted decrements the share policy counter.
func (c *Collector) PolicyDeleted() {
	c.SharePoliciesTotal.Dec()
}
