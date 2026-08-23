package collector

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"network_monitor/stats-collector/internal/config"
)

func TestCollectHealthMetricsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Collector{
		cfg:  config.Config{BackendHealthURL: srv.URL},
		http: srv.Client(),
	}
	ms := c.collectHealthMetrics(context.Background(), time.Now().UTC())
	if len(ms) != 1 {
		t.Fatalf("len=%d", len(ms))
	}
	if ms[0].Type != "health" || ms[0].Name != "up" || ms[0].Value != 1 {
		t.Fatalf("%+v", ms[0])
	}
}

func TestCollectHealthMetricsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &Collector{
		cfg:  config.Config{BackendHealthURL: srv.URL},
		http: srv.Client(),
	}
	ms := c.collectHealthMetrics(context.Background(), time.Now().UTC())
	if ms[0].Value != 0 {
		t.Fatalf("want 0, got %v", ms[0].Value)
	}
}

func TestCollectIngestMetricsPersistsQueueHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"state":"running","buffered_lines":3,"queue_depth":4,"queue_capacity":10,
			"queue_bytes":40,"queue_bytes_capacity":100,"dropped_total":5,
			"buffer_drops_total":6,"circuit_open":true
		}`))
	}))
	defer srv.Close()

	c := &Collector{cfg: config.Config{IngestStatsURL: srv.URL}, http: srv.Client()}
	metrics := c.collectIngestMetrics(context.Background(), time.Now().UTC())
	want := map[string]float64{
		"buffered_lines": 3, "queue_depth": 4, "queue_capacity": 10,
		"queue_bytes": 40, "queue_bytes_capacity": 100, "dropped_total": 5,
		"buffer_drops_total": 6, "circuit_open": 1,
	}
	got := map[string]float64{}
	var up float64
	var upSeen bool
	for _, metric := range metrics {
		if metric.Type == "pipeline" && metric.Target == "ingest" {
			got[metric.Name] = metric.Value
		}
		if metric.Type == "health" && metric.Target == "ingest" && metric.Name == "up" {
			up = metric.Value
			upSeen = true
		}
	}
	if !upSeen || up != 1 {
		t.Fatalf("health.ingest.up=%v seen=%v", up, upSeen)
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("metric %s = %v, want %v", name, got[name], value)
		}
	}
}

func TestCollectIngestMetricsScrapeFailureEmitsDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Collector{cfg: config.Config{IngestStatsURL: srv.URL}, http: srv.Client()}
	metrics := c.collectIngestMetrics(context.Background(), time.Now().UTC())
	if len(metrics) != 1 {
		t.Fatalf("len=%d want health.up only", len(metrics))
	}
	m := metrics[0]
	if m.Type != "health" || m.Target != "ingest" || m.Name != "up" || m.Value != 0 {
		t.Fatalf("%+v", m)
	}
}

func TestCollectSyslogNGMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("dst.network;d_backend_udp;a;dropped;2\ndst.network;d_backend_udp;a;queued;9\n"))
	}))
	defer srv.Close()

	c := &Collector{cfg: config.Config{SyslogStatsURL: srv.URL}, http: srv.Client()}
	metrics := c.collectSyslogNGMetrics(context.Background(), time.Now().UTC())
	got := map[string]float64{}
	var up float64
	for _, m := range metrics {
		if m.Type == "pipeline" && m.Target == "syslogng" {
			got[m.Name] = m.Value
		}
		if m.Type == "health" && m.Target == "syslogng" && m.Name == "up" {
			up = m.Value
		}
	}
	if up != 1 {
		t.Fatalf("up=%v", up)
	}
	if got["dropped_total"] != 2 || got["queued"] != 9 {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectSyslogNGMetricsSkippedWhenUnset(t *testing.T) {
	c := &Collector{cfg: config.Config{}, http: http.DefaultClient}
	if ms := c.collectSyslogNGMetrics(context.Background(), time.Now().UTC()); len(ms) != 0 {
		t.Fatalf("len=%d", len(ms))
	}
}

func TestWriteMetricsSkipsNaN(t *testing.T) {
	c := &Collector{}
	ms := []Metric{
		{Name: "bad", Value: math.NaN()},
		{Name: "inf", Value: math.Inf(1)},
	}
	n, err := c.writeMetrics(context.Background(), ms)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("n=%d", n)
	}
}

func TestCPUPercent(t *testing.T) {
	c := &Collector{cpuPrev: make(map[string]cpuDelta)}
	ts := time.Now()
	if _, ok := c.cpuPercent("x", 1e9, ts); ok {
		t.Fatal("first sample should be undefined")
	}
	pct, ok := c.cpuPercent("x", 1e9+5e8, ts.Add(time.Second))
	if !ok {
		t.Fatal("second sample")
	}
	if pct < 40 || pct > 60 {
		t.Fatalf("pct=%v want ~50", pct)
	}
}
