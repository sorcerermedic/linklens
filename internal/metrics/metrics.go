package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	requests       *prometheus.CounterVec
	duration       *prometheus.HistogramVec
	redirects      prometheus.Counter
	cacheHits      prometheus.Counter
	cacheMisses    prometheus.Counter
	rateLimited    prometheus.Counter
	clicksRecorded prometheus.Counter
	clicksDropped  prometheus.Counter
}

func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		registry: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "linklens_http_requests_total",
			Help: "HTTP requests by method, route pattern and status code.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "linklens_http_request_duration_seconds",
			Help:    "HTTP request latency by route pattern.",
			Buckets: []float64{.001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"method", "route"}),
		redirects: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "linklens_redirects_total", Help: "Successful short link redirects.",
		}),
		cacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "linklens_cache_hits_total", Help: "Link lookups served from cache.",
		}),
		cacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "linklens_cache_misses_total", Help: "Link lookups that fell through to the store.",
		}),
		rateLimited: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "linklens_rate_limited_total", Help: "Requests rejected with 429.",
		}),
		clicksRecorded: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "linklens_clicks_recorded_total", Help: "Click events persisted by the analytics worker.",
		}),
		clicksDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "linklens_clicks_dropped_total", Help: "Click events dropped because the analytics buffer was full.",
		}),
	}
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.requests, m.duration, m.redirects, m.cacheHits, m.cacheMisses,
		m.rateLimited, m.clicksRecorded, m.clicksDropped,
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveRequest(method, route string, status int, d time.Duration) {
	m.requests.WithLabelValues(method, route, strconv.Itoa(status)).Inc()
	m.duration.WithLabelValues(method, route).Observe(d.Seconds())
}

func (m *Metrics) Redirect() { m.redirects.Inc() }

func (m *Metrics) CacheHit() { m.cacheHits.Inc() }

func (m *Metrics) CacheMiss() { m.cacheMisses.Inc() }

func (m *Metrics) RateLimited() { m.rateLimited.Inc() }

func (m *Metrics) ClicksRecorded(n int) { m.clicksRecorded.Add(float64(n)) }

func (m *Metrics) ClickDropped() { m.clicksDropped.Inc() }
