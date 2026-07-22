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
