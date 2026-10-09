// Package metrics records how often each API route is called and by how many
// distinct callers, so that old API versions can be retired once nobody uses
// them any more.
package metrics

import (
	"context"
	"hash/maphash"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	// HealthPath is excluded from the usage metrics.
	HealthPath = "/healthz"

	serviceName = "cosy-template-service"

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

// Recorder collects the usage metrics.
type Recorder struct {
	requests metric.Int64Counter

	mu sync.Mutex
	// seed is random per process: caller addresses are only ever held as a
	// seeded hash, never stored or exported.
	seed     maphash.Seed
	lastSeen map[string]map[uint64]time.Time // route -> caller hash -> last request
	now      func() time.Time
}

// Start returns a Recorder that pushes its metrics over OTLP/HTTP. The
// exporter is configured through the standard OTEL_EXPORTER_OTLP_* environment
// variables; without an endpoint nothing is exported.
func Start(ctx context.Context) (*Recorder, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") == "" {
		return New(noop.NewMeterProvider().Meter(serviceName))
	}

	exporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, err
	}
	// OTEL_SERVICE_NAME and OTEL_RESOURCE_ATTRIBUTES take precedence over the
	// default service name.
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", serviceName)),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	)
	return New(provider.Meter(serviceName))
}

// New returns a Recorder that reports through the given meter.
func New(meter metric.Meter) (*Recorder, error) {
	r := &Recorder{
		seed:     maphash.MakeSeed(),
		lastSeen: make(map[string]map[uint64]time.Time),
		now:      time.Now,
	}

	var err error
	r.requests, err = meter.Int64Counter(
		"cosy_template_service_http_requests",
		metric.WithDescription("HTTP requests by route and status code. Kubernetes probes are not counted."),
	)
	if err != nil {
		return nil, err
	}
	_, err = meter.Int64ObservableGauge(
		"cosy_template_service_unique_clients",
		metric.WithDescription("Distinct callers per route within the look-back window. Counted in memory, so it starts from zero after a restart."),
		metric.WithInt64Callback(r.observeUniqueClients),
	)
	if err != nil {
		return nil, err
	}
	return r, nil
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
		r.requests.Add(context.Background(), 0, requestAttributes(route, http.StatusOK))
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
		r.requests.Add(c.Request.Context(), 1, requestAttributes(route, c.Writer.Status()))
		if route != unmatchedRoute {
			r.seen(route, c.ClientIP())
		}
	}
}

func requestAttributes(route string, status int) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("route", route),
		attribute.String("status", strconv.Itoa(status)),
	)
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

func (r *Recorder) observeUniqueClients(_ context.Context, observer metric.Int64Observer) error {
	now := r.now()

	r.mu.Lock()
	defer r.mu.Unlock()

	for route, clients := range r.lastSeen {
		r.prune(clients, now)
		for _, window := range clientWindows {
			since := now.Add(-window.duration)
			var count int64
			for _, last := range clients {
				if !last.Before(since) {
					count++
				}
			}
			observer.Observe(count, metric.WithAttributes(
				attribute.String("route", route),
				attribute.String("window", window.label),
			))
		}
	}
	return nil
}
