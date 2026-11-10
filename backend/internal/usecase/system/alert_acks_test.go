package system

import (
	"errors"
	"testing"
)

func TestAlertAckStoreEpisodeHideAndClear(t *testing.T) {
	store := NewAlertAckStore()
	stats := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {"queue_depth": 0, "queue_capacity": 1000, "dropped_total": 0},
		},
		Health: map[string]map[string]any{
			"ingest": {"state": 0.0, "state_text": "stopped"},
		},
		Containers: map[string]map[string]float64{},
		Storage:    map[string]map[string]float64{},
	}
	alerts := computeAlerts(stats)
	if !hasAlertCode(alerts, "ingest_idle") {
		t.Fatalf("expected ingest_idle, got %#v", alerts)
	}
	if err := store.Ack("ingest_idle", "ingest", stats, alerts); err != nil {
		t.Fatal(err)
	}
	filtered := store.Filter(alerts, stats)
	if hasAlertCode(filtered, "ingest_idle") {
		t.Fatalf("acked warn should be hidden, got %#v", filtered)
	}

	// Episode ended → ack GC'd, later reappearance shows again.
	stats.Health["ingest"]["state"] = 1.0
	cleared := computeAlerts(stats)
	_ = store.Filter(cleared, stats)
	stats.Health["ingest"]["state"] = 0.0
	again := computeAlerts(stats)
	filtered = store.Filter(again, stats)
	if !hasAlertCode(filtered, "ingest_idle") {
		t.Fatalf("new episode should show alert, got %#v", filtered)
	}
}

func TestAlertAckStoreRejectsError(t *testing.T) {
	store := NewAlertAckStore()
	stats := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{},
		Health: map[string]map[string]any{
			"backend": {"up": 0.0},
		},
		Containers: map[string]map[string]float64{},
		Storage:    map[string]map[string]float64{},
	}
	alerts := computeAlerts(stats)
	if err := store.Ack("backend_down", "backend", stats, alerts); !errors.Is(err, ErrAlertNotAckable) {
		t.Fatalf("err=%v want ErrAlertNotAckable", err)
	}
}

func TestAlertAckStoreBaselineReopen(t *testing.T) {
	store := NewAlertAckStore()
	stats := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {"queue_depth": 0, "queue_capacity": 1000, "dropped_total": 10},
			"rate":   {"drops_per_sec": 0},
		},
		Health:     map[string]map[string]any{},
		Containers: map[string]map[string]float64{},
		Storage:    map[string]map[string]float64{},
	}
	alerts := computeAlerts(stats)
	if err := store.Ack("ingest_dropped_total", "ingest", stats, alerts); err != nil {
		t.Fatal(err)
	}
	if filtered := store.Filter(alerts, stats); hasAlertCode(filtered, "ingest_dropped_total") {
		t.Fatalf("should hide at baseline, got %#v", filtered)
	}

	stats.Pipeline["ingest"]["dropped_total"] = 11
	grown := computeAlerts(stats)
	if filtered := store.Filter(grown, stats); !hasAlertCode(filtered, "ingest_dropped_total") {
		t.Fatalf("growth should reopen, got %#v", filtered)
	}
}

func TestAlertAckStoreUnack(t *testing.T) {
	store := NewAlertAckStore()
	stats := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {"queue_depth": 0, "queue_capacity": 1000, "dropped_total": 3},
			"rate":   {"drops_per_sec": 0},
		},
		Health:     map[string]map[string]any{},
		Containers: map[string]map[string]float64{},
		Storage:    map[string]map[string]float64{},
	}
	alerts := computeAlerts(stats)
	if err := store.Ack("ingest_dropped_total", "ingest", stats, alerts); err != nil {
		t.Fatal(err)
	}
	store.Unack("ingest_dropped_total", "ingest")
	if filtered := store.Filter(alerts, stats); !hasAlertCode(filtered, "ingest_dropped_total") {
		t.Fatalf("unack should show alert, got %#v", filtered)
	}
}

func TestComputeAlertsWarnAreAckable(t *testing.T) {
	stats := SystemStatsResponse{
		Pipeline: map[string]map[string]float64{
			"ingest": {"queue_depth": 0, "queue_capacity": 1000, "dropped_total": 1},
			"rate":   {"drops_per_sec": 0},
		},
		Health:     map[string]map[string]any{},
		Containers: map[string]map[string]float64{},
		Storage:    map[string]map[string]float64{},
	}
	for _, a := range computeAlerts(stats) {
		if a.Level == "warn" && !a.Ackable {
			t.Fatalf("warn %s should be ackable", a.Code)
		}
		if a.Level == "error" && a.Ackable {
			t.Fatalf("error %s must not be ackable", a.Code)
		}
	}
}
