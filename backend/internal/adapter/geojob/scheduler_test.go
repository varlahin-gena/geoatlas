package geojob

import (
	"context"
	"testing"
	"time"

	"network_monitor/internal/model"
)

type stubGeo struct{}

func (stubGeo) Reload(context.Context) error  { return nil }
func (stubGeo) RangeCount() int               { return 0 }
func (stubGeo) Lookup(string) model.GeoLookup { return model.GeoLookup{} }

type stubStore struct {
	edges, geoEdges, enrich, rebuild int
}

func (s *stubStore) BackfillEdgesAgg(context.Context) error {
	s.edges++
	return nil
}
func (s *stubStore) BackfillGeoEdgesAgg(context.Context) error {
	s.geoEdges++
	return nil
}
func (s *stubStore) EnrichLogsMissingGeo(context.Context, GeoResolver, int) (int, error) {
	s.enrich++
	return 0, nil
}
func (s *stubStore) RebuildGeoEdgesLookback(context.Context, int) error {
	s.rebuild++
	return nil
}

func TestSchedulerSerializesAndShutdown(t *testing.T) {
	s := New(stubGeo{}, &stubStore{}, DefaultLookbackDays)

	// Серия Schedule: устаревшие отменяются, Shutdown не должен зависнуть.
	for i := 0; i < 5; i++ {
		s.ScheduleReloadAndEnrich(context.Background(), 2*time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	s.Shutdown(ctx)
	if ctx.Err() != nil {
		t.Fatal("shutdown timed out")
	}
}

func TestSchedulerNilSafe(t *testing.T) {
	var s *Scheduler
	s.ScheduleReloadAndEnrich(context.Background(), time.Second)
	s.ScheduleEnrichOnly(context.Background(), time.Second)
	s.ScheduleMaintenanceBackfill(context.Background(), time.Second)
	s.Shutdown(context.Background())
}

func TestSchedulerMaintenanceUsesStore(t *testing.T) {
	store := &stubStore{}
	s := New(stubGeo{}, store, DefaultLookbackDays)
	s.ScheduleMaintenanceBackfill(context.Background(), time.Hour)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if store.edges >= 1 && store.geoEdges >= 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if store.edges < 1 || store.geoEdges < 1 {
		t.Fatalf("store calls edges=%d geoEdges=%d", store.edges, store.geoEdges)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}
