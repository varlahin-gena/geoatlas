package metrics_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"network_monitor/internal/adapter/metrics"
	"network_monitor/internal/model"
)

type stubIngest struct {
	snap model.IngestLiveStats
}

func (s stubIngest) Stats() model.IngestLiveStats { return s.snap }

func TestMetricsHandlerExposesIngestAndHTTP(t *testing.T) {
	reg := metrics.New(stubIngest{snap: model.IngestLiveStats{
		QueueDepth: 7, QueueCapacity: 100, DroppedTotal: 3, CircuitOpen: true,
	}})
	reg.ObserveHTTP(http.MethodGet, "/api/events", 200, 12*time.Millisecond)
	reg.ObserveInsert(5*time.Millisecond, 10, true)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	text := string(body)
	for _, want := range []string{
		"nm_ingest_queue_depth",
		"nm_ingest_dropped_total",
		"nm_ingest_circuit_open",
		"nm_http_request_duration_seconds",
		"nm_ingest_insert_duration_seconds",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	if !strings.Contains(text, `nm_ingest_queue_depth 7`) {
		t.Fatalf("queue depth not scraped:\n%s", text)
	}
	if !strings.Contains(text, `nm_ingest_circuit_open 1`) {
		t.Fatalf("circuit not open:\n%s", text)
	}
}

func TestSetIngestUpdatesScrape(t *testing.T) {
	reg := metrics.New(nil)
	reg.SetIngest(stubIngest{snap: model.IngestLiveStats{DroppedTotal: 42}})
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), "nm_ingest_dropped_total 42") {
		t.Fatalf("unexpected body:\n%s", rec.Body.String())
	}
}
