// Package metrics holds swosh's Prometheus metrics. Names follow what
// Micrometer exported from the Kotlin service, so existing dashboards keep
// working.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is the set of metrics swosh records.
type Metrics struct {
	registry *prometheus.Registry

	CreateSuccess prometheus.Counter
	CreateFailed  prometheus.Counter
	ExpiredLinks  prometheus.Counter
	HTTPRequests  *prometheus.HistogramVec
}

// Labels are added to every metric, like Micrometer's common tags.
type Labels struct {
	Env  string
	Host string
	App  string
}

// New registers all metrics, plus Go runtime and process metrics, in a new
// registry.
func New(l Labels) *Metrics {
	reg := prometheus.NewRegistry()
	r := prometheus.WrapRegistererWith(prometheus.Labels{"env": l.Env, "host": l.Host, "app": l.App}, reg)
	r.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		registry: reg,
		CreateSuccess: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "create_success_total",
			Help: "Links created through the API.",
		}),
		CreateFailed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "create_failed_total",
			Help: "API requests that did not create a link.",
		}),
		ExpiredLinks: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "expired_links_total",
			Help: "Links deleted because they expired.",
		}),
		HTTPRequests: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_server_requests_seconds",
			Help:    "Time spent serving HTTP requests.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "uri", "status"}),
	}
	r.MustRegister(m.CreateSuccess, m.CreateFailed, m.ExpiredLinks, m.HTTPRequests)
	return m
}

// Handler serves the metrics in the Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
