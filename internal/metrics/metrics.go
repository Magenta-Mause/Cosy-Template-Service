// Package metrics counts how often each API route is called, so that old API
// versions can be retired once nobody uses them any more.
package metrics

import (
	"context"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	serviceName = "cosy-template-service"

	// unmatchedRoute groups every request that hit no registered route, so
	// scanners probing random paths cannot create one series per path.
	unmatchedRoute = "unmatched"
)

// Recorder counts requests per route.
type Recorder struct {
	requests metric.Int64Counter
	ignored  []string
	shutdown func(context.Context) error
}

// Start returns a Recorder that pushes its metrics over OTLP/HTTP. The
// exporter is configured through the standard OTEL_EXPORTER_OTLP_* environment
// variables; without an endpoint nothing is exported. Requests to the ignored
// routes are not counted.
func Start(ctx context.Context, ignoredRoutes ...string) (*Recorder, error) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") == "" {
		return New(noop.NewMeterProvider().Meter(serviceName), ignoredRoutes...)
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

	r, err := New(provider.Meter(serviceName), ignoredRoutes...)
	if err != nil {
		return nil, err
	}
	r.shutdown = provider.Shutdown
	return r, nil
}

// New returns a Recorder that reports through the given meter.
func New(meter metric.Meter, ignoredRoutes ...string) (*Recorder, error) {
	requests, err := meter.Int64Counter(
		"cosy_template_service_http_requests",
		metric.WithDescription("HTTP requests by route and status code. Kubernetes probes are not counted."),
	)
	if err != nil {
		return nil, err
	}
	return &Recorder{requests: requests, ignored: ignoredRoutes}, nil
}

// Shutdown exports what has been counted since the last interval and stops
// the exporter.
func (r *Recorder) Shutdown(ctx context.Context) error {
	if r.shutdown == nil {
		return nil
	}
	return r.shutdown(ctx)
}

// Track makes the given routes report zero from the start, so a route nobody
// calls shows up as 0 rather than as missing data.
func (r *Recorder) Track(routes ...string) {
	for _, route := range routes {
		if slices.Contains(r.ignored, route) {
			continue
		}
		r.requests.Add(context.Background(), 0, requestAttributes(route, http.StatusOK))
	}
}

// Middleware counts every request after it has been handled.
func (r *Recorder) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// The liveness/readiness probes call a real endpoint every few
		// seconds and would otherwise drown out the actual callers.
		if strings.HasPrefix(c.Request.UserAgent(), "kube-probe/") {
			return
		}

		route := c.FullPath()
		if route == "" {
			route = unmatchedRoute
		}
		if slices.Contains(r.ignored, route) {
			return
		}
		r.requests.Add(c.Request.Context(), 1, requestAttributes(route, c.Writer.Status()))
	}
}

func requestAttributes(route string, status int) metric.MeasurementOption {
	return metric.WithAttributes(
		attribute.String("route", route),
		attribute.String("status", strconv.Itoa(status)),
	)
}
