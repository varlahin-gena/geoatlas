package geojob

import (
	"context"
	"testing"
	"time"

	"network_monitor/internal/model"
	"network_monitor/internal/storage"
)

type stubGeo struct{}

func (stubGeo) Reload(context.Context) error  { return nil }
func (stubGeo) RangeCount() int               { return 0 }
func (stubGeo) Lookup(string) model.GeoLookup { return model.GeoLookup{} }

func TestSchedulerSerializesAndShutdown(t *testing.T) {
	s := New(stubGeo{}, nil, storage.DefaultGeoBackfillLookbackDays)

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
