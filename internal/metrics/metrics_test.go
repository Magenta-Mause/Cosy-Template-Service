package metrics

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const healthPath = "/healthz"

func newTestRecorder(t *testing.T) (*Recorder, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	r, err := New(provider.Meter("test"), healthPath)
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
	router.GET(healthPath, ok)
	return router
}

func get(router *gin.Engine, path, userAgent string) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("User-Agent", userAgent)
	router.ServeHTTP(httptest.NewRecorder(), req)
}

// requests returns the request counter's data points keyed by "route status".
func requests(t *testing.T, reader *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatal(err)
	}

	points := map[string]int64{}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			sum, ok := m.Data.(metricdata.Sum[int64])
			if m.Name != "cosy_template_service_http_requests" || !ok {
				t.Fatalf("unexpected metric %s (%T)", m.Name, m.Data)
			}
			for _, dp := range sum.DataPoints {
				route, _ := dp.Attributes.Value("route")
				status, _ := dp.Attributes.Value("status")
				points[fmt.Sprintf("%s %s", route.AsString(), status.AsString())] = dp.Value
			}
		}
	}
	return points
}

func assertRequests(t *testing.T, reader *sdkmetric.ManualReader, want map[string]int64) {
	t.Helper()
	if got := requests(t, reader); !maps.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRequestsAreCountedPerRoute(t *testing.T) {
	r, reader := newTestRecorder(t)
	router := newTestRouter(r)

	get(router, "/v1/templates", "ReactorNetty/1.2")
	get(router, "/v3/templates", "ReactorNetty/1.2")
	get(router, "/v3/templates", "ReactorNetty/1.2")

	assertRequests(t, reader, map[string]int64{
		"/v1/templates 200": 1,
		"/v3/templates 200": 2,
	})
}

func TestProbesAndIgnoredRoutesAreNotCounted(t *testing.T) {
	r, reader := newTestRecorder(t)
	router := newTestRouter(r)

	get(router, "/v1/templates", "kube-probe/1.33")
	get(router, healthPath, "curl/8")

	assertRequests(t, reader, map[string]int64{})
}

func TestUnknownPathsShareOneRoute(t *testing.T) {
	r, reader := newTestRecorder(t)
	router := newTestRouter(r)

	get(router, "/.env", "scanner")
	get(router, "/wp-config.php", "scanner")

	assertRequests(t, reader, map[string]int64{"unmatched 404": 2})
}

func TestTrackedRoutesReportZero(t *testing.T) {
	r, reader := newTestRecorder(t)
	r.Track("/v2/templates", healthPath)

	assertRequests(t, reader, map[string]int64{"/v2/templates 200": 0})
}
