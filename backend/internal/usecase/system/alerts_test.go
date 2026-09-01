package system

import (
	"sync"
	"testing"
	"time"
)

func TestComputeAlertsIngestDropping(t *testing.T) {
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {"queue_depth": 0, "queue_capacity": 1000, "dropped_total": 10},
			"rate":   {"drops_per_sec": 0},
		},
		Health: map[string]map[string]any{}, Containers: map[string]map[string]float64{}, Storage: map[string]map[string]float64{},
	}
	alerts := computeAlerts(base)
	if !hasAlertCode(alerts, "ingest_dropped_total") {
		t.Fatalf("expected ingest_dropped_total, got %#v", alerts)
	}
	base.Pipeline["rate"]["drops_per_sec"] = 5
	alerts = computeAlerts(base)
	if !hasAlertCode(alerts, "ingest_dropping") {
		t.Fatalf("expected ingest_dropping, got %#v", alerts)
	}
	if hasAlertCode(alerts, "ingest_dropped_total") {
		t.Fatal("active drop rate should supersede total-only warn")
	}
	base.Pipeline["rate"]["drops_per_sec"] = 150
	if alerts = computeAlerts(base); !hasAlertCode(alerts, "ingest_dropping_critical") {
		t.Fatalf("expected ingest_dropping_critical, got %#v", alerts)
	}
}

func TestComputeAlertsEdgesAggRebuilding(t *testing.T) {
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{}, Health: map[string]map[string]any{},
		Containers: map[string]map[string]float64{}, Storage: map[string]map[string]float64{},
		EdgesAgg: EdgesAggView{State: "running", Phase: "schema", Message: "rebuilding", MapSource: "traffic_logs"},
	}
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "edges_agg_rebuilding") {
		t.Fatalf("expected edges_agg_rebuilding, got %#v", alerts)
	}
	base.EdgesAgg.Phase, base.EdgesAgg.DaysDone, base.EdgesAgg.DaysTotal = "backfill", 2, 5
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "edges_agg_backfill") {
		t.Fatalf("expected edges_agg_backfill, got %#v", alerts)
	}
	base.EdgesAgg = EdgesAggView{State: "error", Message: "boom"}
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "edges_agg_error") {
		t.Fatalf("expected edges_agg_error, got %#v", alerts)
	}
}

func TestComputeAlertsEdgesAggRunningLong(t *testing.T) {
	now := time.Now().UTC()
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{}, Health: map[string]map[string]any{},
		Containers: map[string]map[string]float64{}, Storage: map[string]map[string]float64{},
		EdgesAgg: EdgesAggView{
			State: "running", Phase: "backfill", DaysDone: 1, DaysTotal: 10,
			StartedAt: now.Add(-3 * time.Hour), MapSource: "traffic_logs",
		},
	}
	alerts := edgesAggAlerts(base.EdgesAgg, now)
	if !hasAlertCode(alerts, "edges_agg_running_long") {
		t.Fatalf("expected edges_agg_running_long, got %#v", alerts)
	}
	base.EdgesAgg.StartedAt = now.Add(-7 * time.Hour)
	alerts = edgesAggAlerts(base.EdgesAgg, now)
	if !hasAlertCode(alerts, "edges_agg_running_stuck") {
		t.Fatalf("expected edges_agg_running_stuck, got %#v", alerts)
	}
}

func TestMergeLiveIngestStatsDropsPerSec(t *testing.T) {
	rates := &RateSampler{
		prevDrop: 100,
		prevTS:   time.Now().Add(-2 * time.Second),
	}

	resp := &SystemStatsResponse{Pipeline: map[string]map[string]float64{}, Health: map[string]map[string]any{}}
	mergeLiveIngestStats(resp, IngestSnapshot{State: "running", DroppedTotal: 300, ReceivedTotal: 0}, rates)
	rate := resp.Pipeline["rate"]["drops_per_sec"]
	if rate < 50 {
		t.Fatalf("drops_per_sec=%v want roughly 100", rate)
	}
	if resp.Pipeline["ingest"]["dropped_total"] != 300 {
		t.Fatalf("dropped_total=%v", resp.Pipeline["ingest"]["dropped_total"])
	}
	alerts := computeAlerts(*resp)
	if !hasAlertCode(alerts, "ingest_dropping_critical") && !hasAlertCode(alerts, "ingest_dropping") {
		t.Fatalf("expected dropping alert from merged rate, got %#v", alerts)
	}
}

func hasAlertCode(alerts []Alert, code string) bool {
	for _, alert := range alerts {
		if alert.Code == code {
			return true
		}
	}
	return false
}

func TestClassifyIngestQueueCritical(t *testing.T) {
	st, reasons, _ := classifyIngest(IngestSnapshot{
		State: "running", QueueDepth: 950, QueueCapacity: 1000,
	}, &RateSampler{}, DefaultIngestSLO())
	if st != "overloaded" {
		t.Fatalf("status=%s want overloaded", st)
	}
	if !containsReason(reasons, "queue_critical") {
		t.Fatalf("reasons=%v", reasons)
	}
}

func TestClassifyIngestQueueBytesCritical(t *testing.T) {
	st, reasons, _ := classifyIngest(IngestSnapshot{
		State: "running", QueueDepth: 10, QueueCapacity: 1000,
		QueueBytes: 95 << 20, QueueBytesCapacity: 100 << 20,
	}, &RateSampler{}, DefaultIngestSLO())
	if st != "overloaded" {
		t.Fatalf("status=%s want overloaded", st)
	}
	if !containsReason(reasons, "queue_bytes_critical") {
		t.Fatalf("reasons=%v", reasons)
	}
}

func TestComputeAlertsQueueBytes(t *testing.T) {
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {
				"queue_depth": 0, "queue_capacity": 1000,
				"queue_bytes": 80, "queue_bytes_capacity": 100,
				"dropped_total": 0,
			},
			"rate": {"drops_per_sec": 0},
		},
		Health: map[string]map[string]any{}, Containers: map[string]map[string]float64{}, Storage: map[string]map[string]float64{},
	}
	alerts := computeAlerts(base)
	if !hasAlertCode(alerts, "ingest_queue_bytes_high") {
		t.Fatalf("expected ingest_queue_bytes_high, got %#v", alerts)
	}
	base.Pipeline["ingest"]["queue_bytes"] = 95
	if alerts = computeAlerts(base); !hasAlertCode(alerts, "ingest_queue_bytes_critical") {
		t.Fatalf("expected ingest_queue_bytes_critical, got %#v", alerts)
	}
}

func TestComputeAlertsBufferHighNeedsPressureSignal(t *testing.T) {
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {
				"buffered_lines": 25000,
				"queue_depth":     2500,
				"queue_capacity":  200000,
				"lag_sec":         12,
				"dropped_total":   0,
			},
			"rate": {"drops_per_sec": 0, "buffer_drops_per_sec": 0},
		},
		Health: map[string]map[string]any{}, Containers: map[string]map[string]float64{}, Storage: map[string]map[string]float64{},
	}
	if alerts := computeAlerts(base); hasAlertCode(alerts, "ingest_buffer_high") || hasAlertCode(alerts, "ingest_buffer_critical") {
		t.Fatalf("buffer-only backlog should not alert, got %#v", alerts)
	}

	base.Pipeline["ingest"]["lag_sec"] = 20
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "ingest_buffer_high") {
		t.Fatalf("expected ingest_buffer_high with lag pressure, got %#v", alerts)
	}
}

func TestComputeAlertsBufferCriticalNeedsPressureSignal(t *testing.T) {
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {
				"buffered_lines": 120000,
				"queue_depth":     5000,
				"queue_capacity":  200000,
				"lag_sec":         10,
			},
			"rate": {"drops_per_sec": 0, "buffer_drops_per_sec": 0},
		},
		Health: map[string]map[string]any{}, Containers: map[string]map[string]float64{}, Storage: map[string]map[string]float64{},
	}
	if alerts := computeAlerts(base); hasAlertCode(alerts, "ingest_buffer_critical") {
		t.Fatalf("buffer-only critical backlog should not alert, got %#v", alerts)
	}

	base.Pipeline["ingest"]["queue_depth"] = 25000
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "ingest_buffer_critical") {
		t.Fatalf("expected ingest_buffer_critical with queue pressure, got %#v", alerts)
	}
}

func TestClassifyIngestHealthy(t *testing.T) {
	st, reasons, _ := classifyIngest(IngestSnapshot{
		State: "running", QueueDepth: 10, QueueCapacity: 1000,
	}, &RateSampler{}, DefaultIngestSLO())
	if st != "healthy" {
		t.Fatalf("status=%s want healthy, reasons=%v", st, reasons)
	}
}

func TestClassifyIngestBufferDropping(t *testing.T) {
	rates := &RateSampler{}
	_, _, _ = classifyIngest(IngestSnapshot{
		State: "running", QueueDepth: 1, QueueCapacity: 1000, BufferDropsTotal: 0,
	}, rates, DefaultIngestSLO())
	time.Sleep(30 * time.Millisecond)
	st, reasons, _ := classifyIngest(IngestSnapshot{
		State: "running", QueueDepth: 1, QueueCapacity: 1000, BufferDropsTotal: 5,
	}, rates, DefaultIngestSLO())
	if st != "degraded" && st != "overloaded" {
		t.Fatalf("status=%s want degraded/overloaded, reasons=%v", st, reasons)
	}
	if !containsReason(reasons, "dropping") && !containsReason(reasons, "dropping_critical") {
		t.Fatalf("reasons=%v want dropping*", reasons)
	}
}

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}

func TestComputeAlertsSyslogNG(t *testing.T) {
	base := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"syslogng": {"dropped_total": 4, "queued": 10, "drops_per_sec": 0},
		},
		Health:     map[string]map[string]any{"syslogng": {"up": 1.0}},
		Containers: map[string]map[string]float64{"syslog-ng": {"mem_bytes": 1}},
		Storage:    map[string]map[string]float64{},
		InstallProfile: &CapacityProfile{
			Limits:   ProfileLimits{SyslogNG: ProfileSyslogLimits{FifoSize: 50}},
			Capacity: ProfileCapacity{},
		},
	}
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "syslogng_dropped_total") {
		t.Fatalf("expected syslogng_dropped_total, got %#v", alerts)
	}
	base.Pipeline["syslogng"]["drops_per_sec"] = 5
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "syslogng_dropping") {
		t.Fatalf("expected syslogng_dropping, got %#v", alerts)
	}
	base.Pipeline["syslogng"]["drops_per_sec"] = 150
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "syslogng_dropping_critical") {
		t.Fatalf("expected syslogng_dropping_critical, got %#v", alerts)
	}
	base.Pipeline["syslogng"]["drops_per_sec"] = 0
	base.Pipeline["syslogng"]["dropped_total"] = 0
	base.Pipeline["syslogng"]["queued"] = 80
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "syslogng_queue_high") {
		t.Fatalf("expected syslogng_queue_high, got %#v", alerts)
	}
	base.Pipeline["syslogng"]["queued"] = 95
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "syslogng_queue_critical") {
		t.Fatalf("expected syslogng_queue_critical, got %#v", alerts)
	}
	base.Health["syslogng"]["up"] = 0.0
	if alerts := computeAlerts(base); !hasAlertCode(alerts, "syslogng_down") {
		t.Fatalf("expected syslogng_down, got %#v", alerts)
	}
	delete(base.Containers, "syslog-ng")
	if alerts := computeAlerts(base); hasAlertCode(alerts, "syslogng_down") {
		t.Fatalf("syslogng_down without container metrics: %#v", alerts)
	}
}

func TestMergeLiveIngestStatsConcurrent(t *testing.T) {
	rates := &RateSampler{}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(n int64) {
			defer wg.Done()
			resp := &SystemStatsResponse{
				Pipeline: map[string]map[string]float64{},
				Health:   map[string]map[string]any{},
			}
			mergeLiveIngestStats(resp, IngestSnapshot{
				ReceivedTotal: n,
				UDPReceived:   n / 2,
				TCPReceived:   n / 2,
				State:         "running",
			}, rates)
		}(int64(i * 100))
	}
	wg.Wait()
}

func TestMergeLiveSyslogNG(t *testing.T) {
	resp := &SystemStatsResponse{
		Pipeline: map[string]map[string]float64{"syslogng": {"queued": 1}},
		Health:   map[string]map[string]any{},
	}
	mergeLiveSyslogNG(resp, SyslogNGSnapshot{Up: false}, &RateSampler{})
	if resp.Health["syslogng"]["up"].(float64) != 0 {
		t.Fatalf("up=%v", resp.Health["syslogng"]["up"])
	}
	if resp.Pipeline["syslogng"]["queued"] != 1 {
		t.Fatal("failed scrape must not wipe CH queued")
	}
	mergeLiveSyslogNG(resp, SyslogNGSnapshot{Up: true, DroppedTotal: 4, Queued: 9, ProcessedTotal: 20, UDPProcessed: 12, TCPProcessed: 8}, &RateSampler{})
	p := resp.Pipeline["syslogng"]
	if p["dropped_total"] != 4 || p["queued"] != 9 || p["udp_processed"] != 12 {
		t.Fatalf("%#v", p)
	}
	if resp.Health["syslogng"]["up"].(float64) != 1 {
		t.Fatal("up")
	}
}
