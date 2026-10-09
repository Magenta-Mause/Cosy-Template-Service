// Package metrics records how often each API route is called and by how many
// distinct callers, so that old API versions can be retired once nobody uses
// them any more.
package metrics

import (
	"hash/maphash"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	// HealthPath is excluded from the usage metrics.
	HealthPath = "/healthz"

	// unmatchedRoute groups every request that hit no registered route, so
	// scanners probing random paths cannot create one series per path.
	unmatchedRoute = "unmatched"

	// maxClientsPerRoute bounds the memory the distinct-caller tracking can use.
	maxClientsPerRoute = 50_000
)

// clientWindows are the look-back windows reported by the unique-clients gauge.
var clientWindows = []struct {
	label    string
	duration time.Duration
}{
	{"1d", 24 * time.Hour},
	{"7d", 7 * 24 * time.Hour},
	{"30d", 30 * 24 * time.Hour},
}

var uniqueClientsDesc = prometheus.NewDesc(
	"cosy_template_service_unique_clients",
	"Distinct callers per route within the look-back window. Counted in memory, so it starts from zero after a restart.",
	[]string{"route", "window"}, nil,
)

// Recorder collects the usage metrics and serves them.
type Recorder struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec

	mu sync.Mutex
	// seed is random per process: caller addresses are only ever held as a
	// seeded hash, never stored or exported.
	seed     maphash.Seed
	lastSeen map[string]map[uint64]time.Time // route -> caller hash -> last request
	now      func() time.Time
}

func New() *Recorder {
	r := &Recorder{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "cosy_template_service_http_requests_total",
			Help: "HTTP requests by route and status code. Kubernetes probes are not counted.",
		}, []string{"route", "status"}),
		seed:     maphash.MakeSeed(),
		lastSeen: make(map[string]map[uint64]time.Time),
		now:      time.Now,
	}
	r.registry.MustRegister(r.requests, r)
	return r
}

// Track makes the given routes report zero from the start, so a route nobody
// calls shows up as 0 rather than as missing data.
func (r *Recorder) Track(routes ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, route := range routes {
		if route == HealthPath {
			continue
		}
		r.requests.WithLabelValues(route, strconv.Itoa(http.StatusOK))
		if r.lastSeen[route] == nil {
			r.lastSeen[route] = make(map[uint64]time.Time)
		}
	}
}

// Middleware counts every request after it has been handled.
func (r *Recorder) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// The liveness/readiness probes call a real endpoint every few
		// seconds and would otherwise drown out the actual callers.
		if c.FullPath() == HealthPath || strings.HasPrefix(c.Request.UserAgent(), "kube-probe/") {
			return
		}

		route := c.FullPath()
		if route == "" {
			route = unmatchedRoute
		}
		r.requests.WithLabelValues(route, strconv.Itoa(c.Writer.Status())).Inc()
		if route != unmatchedRoute {
			r.seen(route, c.ClientIP())
		}
	}
}

// Handler serves the metrics in the Prometheus text format.
func (r *Recorder) Handler() http.Handler {
	return promhttp.HandlerFor(r.registry, promhttp.HandlerOpts{})
}

func (r *Recorder) seen(route, client string) {
	now := r.now()
	key := maphash.String(r.seed, client)

	r.mu.Lock()
	defer r.mu.Unlock()

	clients := r.lastSeen[route]
	if clients == nil {
		clients = make(map[uint64]time.Time)
		r.lastSeen[route] = clients
	}
	if _, known := clients[key]; !known && len(clients) >= maxClientsPerRoute {
		r.prune(clients, now)
		if len(clients) >= maxClientsPerRoute {
			return
		}
	}
	clients[key] = now
}

// prune drops callers that are older than the longest window.
func (r *Recorder) prune(clients map[uint64]time.Time, now time.Time) {
	oldest := now.Add(-clientWindows[len(clientWindows)-1].duration)
	for key, last := range clients {
		if last.Before(oldest) {
			delete(clients, key)
		}
	}
}

// Describe implements prometheus.Collector.
func (r *Recorder) Describe(ch chan<- *prometheus.Desc) {
	ch <- uniqueClientsDesc
}

// Collect implements prometheus.Collector.
func (r *Recorder) Collect(ch chan<- prometheus.Metric) {
	now := r.now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for route, clients := range r.lastSeen {
		r.prune(clients, now)
		for _, window := range clientWindows {
			since := now.Add(-window.duration)
			count := 0
			for _, last := range clients {
				if !last.Before(since) {
					count++
				}
			}
			ch <- prometheus.MustNewConstMetric(
				uniqueClientsDesc, prometheus.GaugeValue, float64(count), route, window.label,
			)
		}
	}
}
