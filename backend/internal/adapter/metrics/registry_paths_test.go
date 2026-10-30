package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoatlas/internal/adapter/metrics"
)

func TestNilRegistryAndAnomalyObservers(t *testing.T) {
	var nilR *metrics.Registry
	nilR.ObserveHTTP("GET", "/", 200, time.Millisecond)
	nilR.IncInFlight()
	nilR.DecInFlight()
	nilR.ObserveInsert(time.Millisecond, 1, false)
	nilR.ObserveScan(time.Millisecond, 1, "")
	nilR.IncDetected("x", "high")
	nilR.IncScanError("x")
	nilR.SetIngest(nil)
	rec := httptest.NewRecorder()
	nilR.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d", rec.Code)
	}

	reg := metrics.New(nil)
	reg.IncInFlight()
	reg.DecInFlight()
	reg.ObserveInsert(time.Millisecond, 0, false)
	reg.ObserveInsert(time.Millisecond, 5, false)
	reg.ObserveScan(time.Millisecond, 0, "circuit")
	reg.ObserveScan(time.Millisecond, 3, "")
	reg.IncDetected("port_scan", "medium")
	reg.IncScanError("insert")

	rec2 := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec2.Body.String()
	for _, want := range []string{
		"ga_anomaly_skipped_tick_total",
		"ga_anomaly_detected_total",
		"ga_anomaly_scan_errors_total",
		"ga_anomaly_insert_total",
		"ga_http_requests_in_flight",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in\n%s", want, body)
		}
	}
}
