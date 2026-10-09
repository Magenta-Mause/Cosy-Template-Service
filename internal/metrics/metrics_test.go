package metrics

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const (
	requestsMetric = "cosy_template_service_http_requests"
	clientsMetric  = "cosy_template_service_unique_clients"
)

func newTestRecorder(t *testing.T) (*Recorder, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	r, err := New(provider.Meter("test"))
	if err != nil {
		t.Fatal(err)
	}
	return r, reader
}

func newTestRouter(r *Recorder) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(r.Middleware())
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	router.GET("/v1/templates", ok)
	router.GET("/v3/templates", ok)
	router.GET(HealthPath, ok)
	return router
}

func get(router *gin.Engine, path, client, userAgent string) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = client + ":1234"
	req.Header.Set("User-Agent", userAgent)
	router.ServeHTTP(httptest.NewRecorder(), req)
}

// collect returns every data point of the named metric, keyed by its two
// attribute values, e.g. "/v1/templates 200" or "/v1/templates 7d".
func collect(t *testing.T, reader *sdkmetric.ManualReader, name, attrA, attrB string) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}

	points := map[string]int64{}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != name {
				continue
			}
			var dataPoints []metricdata.DataPoint[int64]
			switch data := m.Data.(type) {
			case metricdata.Sum[int64]:
				dataPoints = data.DataPoints
			case metricdata.Gauge[int64]:
				dataPoints = data.DataPoints
			}
			for _, dp := range dataPoints {
				a, _ := dp.Attributes.Value(attribute.Key(attrA))
				b, _ := dp.Attributes.Value(attribute.Key(attrB))
				points[fmt.Sprintf("%s %s", a.AsString(), b.AsString())] = dp.Value
			}
		}
	}
	return points
}

func assertPoints(t *testing.T, got, want map[string]int64) {
	t.Helper()
	if !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRequestsAreCountedPerRoute(t *testing.T) {
	r, reader := newTestRecorder(t)
	router := newTestRouter(r)

	get(router, "/v1/templates", "10.0.0.1", "ReactorNetty/1.2")
	get(router, "/v3/templates", "10.0.0.1", "ReactorNetty/1.2")
	get(router, "/v3/templates", "10.0.0.2", "ReactorNetty/1.2")

	assertPoints(t, collect(t, reader, requestsMetric, "route", "status"), map[string]int64{
		"/v1/templates 200": 1,
		"/v3/templates 200": 2,
	})
}

func TestProbesAndHealthChecksAreNotCounted(t *testing.T) {
	r, reader := newTestRecorder(t)
	router := newTestRouter(r)

	get(router, "/v1/templates", "10.42.0.1", "kube-probe/1.33")
	get(router, HealthPath, "10.42.0.1", "curl/8")

	assertPoints(t, collect(t, reader, requestsMetric, "route", "status"), map[string]int64{})
	assertPoints(t, collect(t, reader, clientsMetric, "route", "window"), map[string]int64{})
}

func TestUnknownPathsShareOneRoute(t *testing.T) {
	r, reader := newTestRecorder(t)
	router := newTestRouter(r)

	get(router, "/.env", "203.0.113.7", "scanner")
	get(router, "/wp-config.php", "203.0.113.7", "scanner")

	assertPoints(t, collect(t, reader, requestsMetric, "route", "status"), map[string]int64{
		"unmatched 404": 2,
	})
	// Unmatched requests are not tracked as clients.
	assertPoints(t, collect(t, reader, clientsMetric, "route", "window"), map[string]int64{})
}

func TestUniqueClientsPerWindow(t *testing.T) {
	r, reader := newTestRecorder(t)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	router := newTestRouter(r)

	// Two callers ten days ago, one of them again today.
	now = now.Add(-10 * 24 * time.Hour)
	get(router, "/v1/templates", "10.0.0.1", "old-backend")
	get(router, "/v1/templates", "10.0.0.2", "old-backend")
	now = now.Add(10 * 24 * time.Hour)
	get(router, "/v1/templates", "10.0.0.1", "old-backend")
	get(router, "/v1/templates", "10.0.0.1", "old-backend")

	assertPoints(t, collect(t, reader, clientsMetric, "route", "window"), map[string]int64{
		"/v1/templates 1d":  1,
		"/v1/templates 7d":  1,
		"/v1/templates 30d": 2,
	})

	// After 30 days without a call the route reports nobody.
	now = now.Add(31 * 24 * time.Hour)
	assertPoints(t, collect(t, reader, clientsMetric, "route", "window"), map[string]int64{
		"/v1/templates 1d":  0,
		"/v1/templates 7d":  0,
		"/v1/templates 30d": 0,
	})
}

func TestTrackedRoutesReportZero(t *testing.T) {
	r, reader := newTestRecorder(t)
	r.Track("/v2/templates", HealthPath)

	assertPoints(t, collect(t, reader, requestsMetric, "route", "status"), map[string]int64{
		"/v2/templates 200": 0,
	})
	assertPoints(t, collect(t, reader, clientsMetric, "route", "window"), map[string]int64{
		"/v2/templates 1d":  0,
		"/v2/templates 7d":  0,
		"/v2/templates 30d": 0,
	})
}
