package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

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

func TestRequestsAreCountedPerRoute(t *testing.T) {
	r := New()
	router := newTestRouter(r)

	get(router, "/v1/templates", "10.0.0.1", "ReactorNetty/1.2")
	get(router, "/v3/templates", "10.0.0.1", "ReactorNetty/1.2")
	get(router, "/v3/templates", "10.0.0.2", "ReactorNetty/1.2")

	want := `
# HELP cosy_template_service_http_requests_total HTTP requests by route and status code. Kubernetes probes are not counted.
# TYPE cosy_template_service_http_requests_total counter
cosy_template_service_http_requests_total{route="/v1/templates",status="200"} 1
cosy_template_service_http_requests_total{route="/v3/templates",status="200"} 2
`
	if err := testutil.CollectAndCompare(r.requests, strings.NewReader(want)); err != nil {
		t.Error(err)
	}
}

func TestProbesAndHealthChecksAreNotCounted(t *testing.T) {
	r := New()
	router := newTestRouter(r)

	get(router, "/v1/templates", "10.42.0.1", "kube-probe/1.33")
	get(router, HealthPath, "10.42.0.1", "curl/8")

	if got := testutil.CollectAndCount(r.requests); got != 0 {
		t.Errorf("expected no request series, got %d", got)
	}
	if got := testutil.CollectAndCount(r); got != 0 {
		t.Errorf("expected no unique-client series, got %d", got)
	}
}

func TestUnknownPathsShareOneRoute(t *testing.T) {
	r := New()
	router := newTestRouter(r)

	get(router, "/.env", "203.0.113.7", "scanner")
	get(router, "/wp-config.php", "203.0.113.7", "scanner")

	want := `
# HELP cosy_template_service_http_requests_total HTTP requests by route and status code. Kubernetes probes are not counted.
# TYPE cosy_template_service_http_requests_total counter
cosy_template_service_http_requests_total{route="unmatched",status="404"} 2
`
	if err := testutil.CollectAndCompare(r.requests, strings.NewReader(want)); err != nil {
		t.Error(err)
	}
	if got := testutil.CollectAndCount(r); got != 0 {
		t.Errorf("unmatched requests must not be tracked as clients, got %d series", got)
	}
}

func TestUniqueClientsPerWindow(t *testing.T) {
	r := New()
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

	want := `
# HELP cosy_template_service_unique_clients Distinct callers per route within the look-back window. Counted in memory, so it starts from zero after a restart.
# TYPE cosy_template_service_unique_clients gauge
cosy_template_service_unique_clients{route="/v1/templates",window="1d"} 1
cosy_template_service_unique_clients{route="/v1/templates",window="7d"} 1
cosy_template_service_unique_clients{route="/v1/templates",window="30d"} 2
`
	if err := testutil.CollectAndCompare(r, strings.NewReader(want)); err != nil {
		t.Error(err)
	}

	// After 30 days without a call the route reports nobody.
	now = now.Add(31 * 24 * time.Hour)
	want = `
# HELP cosy_template_service_unique_clients Distinct callers per route within the look-back window. Counted in memory, so it starts from zero after a restart.
# TYPE cosy_template_service_unique_clients gauge
cosy_template_service_unique_clients{route="/v1/templates",window="1d"} 0
cosy_template_service_unique_clients{route="/v1/templates",window="7d"} 0
cosy_template_service_unique_clients{route="/v1/templates",window="30d"} 0
`
	if err := testutil.CollectAndCompare(r, strings.NewReader(want)); err != nil {
		t.Error(err)
	}
}

func TestTrackedRoutesReportZero(t *testing.T) {
	r := New()
	r.Track("/v2/templates", HealthPath)

	want := `
# HELP cosy_template_service_http_requests_total HTTP requests by route and status code. Kubernetes probes are not counted.
# TYPE cosy_template_service_http_requests_total counter
cosy_template_service_http_requests_total{route="/v2/templates",status="200"} 0
`
	if err := testutil.CollectAndCompare(r.requests, strings.NewReader(want)); err != nil {
		t.Error(err)
	}
	if got := testutil.CollectAndCount(r); got != len(clientWindows) {
		t.Errorf("expected %d unique-client series, got %d", len(clientWindows), got)
	}
}
