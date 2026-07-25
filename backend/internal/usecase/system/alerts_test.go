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

func TestMergeLiveIngestStatsDropsPerSec(t *testing.T) {
	prevIngestMu.Lock()
	prevIngestRecv, prevIngestUDPRecv, prevIngestTCPRecv, prevIngestDropped = 0, 0, 0, 100
	prevIngestTS = time.Now().Add(-2 * time.Second)
	prevIngestMu.Unlock()

	resp := &SystemStatsResponse{Pipeline: map[string]map[string]float64{}, Health: map[string]map[string]any{}}
	mergeLiveIngestStats(resp, IngestSnapshot{State: "running", DroppedTotal: 300, ReceivedTotal: 0})
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
	})
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
	})
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

func TestClassifyIngestHealthy(t *testing.T) {
	st, reasons, _ := classifyIngest(IngestSnapshot{
		State: "running", QueueDepth: 10, QueueCapacity: 1000,
	})
	if st != "healthy" {
		t.Fatalf("status=%s want healthy, reasons=%v", st, reasons)
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

func TestMergeLiveIngestStatsConcurrent(t *testing.T) {
	prevIngestMu.Lock()
	prevIngestRecv = 0
	prevIngestUDPRecv = 0
	prevIngestTCPRecv = 0
	prevIngestDropped = 0
	prevIngestTS = time.Time{}
	prevIngestMu.Unlock()

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
			})
		}(int64(i * 100))
	}
	wg.Wait()
}
