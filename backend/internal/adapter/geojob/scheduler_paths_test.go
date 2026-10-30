package geojob

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"geoatlas/internal/adapter/heavytask"
	"geoatlas/internal/model"
)

type countingGeo struct {
	reloads atomic.Int64
	ranges  int
	err     error
}

func (g *countingGeo) Reload(context.Context) error {
	g.reloads.Add(1)
	return g.err
}
func (g *countingGeo) RangeCount() int               { return g.ranges }
func (g *countingGeo) Lookup(string) model.GeoLookup { return model.GeoLookup{} }

type errStore struct {
	stubStore
	edgesErr, geoErr, enrichErr error
	enrichN                     int
}

func (s *errStore) BackfillEdgesAgg(context.Context) error {
	s.edges.Add(1)
	return s.edgesErr
}
func (s *errStore) BackfillGeoEdgesAgg(context.Context) error {
	s.geoEdges.Add(1)
	return s.geoErr
}
func (s *errStore) EnrichLogsMissingGeo(context.Context, GeoResolver, int) (int, error) {
	s.enrich.Add(1)
	return s.enrichN, s.enrichErr
}

func TestNewLookbackDefault(t *testing.T) {
	s := New(nil, nil, -1)
	if s.lookbackDays != DefaultLookbackDays {
		t.Fatal(s.lookbackDays)
	}
	var nilS *Scheduler
	nilS.SetLimiter(heavytask.New(1))
	nilS.SetGate(staticSkip(""))
}

func TestScheduleNilGuards(t *testing.T) {
	s := New(nil, nil, 0)
	s.ScheduleEnrichOnly(context.Background(), time.Second)
	s.ScheduleReloadAndEnrich(context.Background(), time.Second)
	// maintenance allowed without geo
	store := &stubStore{}
	s = New(nil, store, 0)
	s.ScheduleMaintenanceBackfill(context.Background(), time.Second) // bumps to 6h
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && store.edges.Load() < 1 {
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

func TestEnrichAndReloadPaths(t *testing.T) {
	geo := &countingGeo{ranges: 2}
	store := &errStore{enrichN: 3}
	s := New(geo, store, 3)
	s.SetLimiter(heavytask.New(1))
	s.ScheduleReloadAndEnrich(context.Background(), 2*time.Second)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if geo.reloads.Load() >= 1 && store.enrich.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if geo.reloads.Load() < 1 || store.enrich.Load() < 1 {
		t.Fatalf("reload=%d enrich=%d", geo.reloads.Load(), store.enrich.Load())
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)

	store2 := &errStore{enrichErr: errors.New("enrich boom")}
	s2 := New(geo, store2, 1)
	s2.ScheduleEnrichOnly(context.Background(), time.Second)
	time.Sleep(50 * time.Millisecond)
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	s2.Shutdown(ctx2)

	geo.err = errors.New("reload boom")
	store3 := &errStore{}
	s3 := New(geo, store3, 1)
	s3.ScheduleReloadAndEnrich(context.Background(), time.Second)
	time.Sleep(50 * time.Millisecond)
	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Second)
	defer cancel3()
	s3.Shutdown(ctx3)
}

func TestMaintenanceErrorAndEnrich(t *testing.T) {
	geo := &countingGeo{ranges: 1}
	store := &errStore{edgesErr: errors.New("edges fail")}
	s := New(geo, store, 1)
	s.ScheduleMaintenanceBackfill(context.Background(), time.Hour)
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)
	if store.edges.Load() < 1 || store.geoEdges.Load() != 0 {
		t.Fatalf("edges=%d geo=%d", store.edges.Load(), store.geoEdges.Load())
	}

	storeOK := &errStore{enrichN: 1}
	s2 := New(geo, storeOK, 1)
	s2.ScheduleMaintenanceBackfill(context.Background(), time.Hour)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if storeOK.edges.Load() >= 1 && storeOK.geoEdges.Load() >= 1 && storeOK.enrich.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if storeOK.enrich.Load() < 1 {
		t.Fatal("expected enrich after maintenance")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	s2.Shutdown(ctx2)

	storeGeoFail := &errStore{geoErr: errors.New("geo fail")}
	s3 := New(geo, storeGeoFail, 1)
	s3.ScheduleMaintenanceBackfill(context.Background(), time.Hour)
	time.Sleep(50 * time.Millisecond)
	ctx3, cancel3 := context.WithTimeout(context.Background(), time.Second)
	defer cancel3()
	s3.Shutdown(ctx3)
}

func TestHeavyAcquireCancel(t *testing.T) {
	lim := heavytask.New(1)
	if err := lim.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	store := &stubStore{}
	s := New(stubGeo{}, store, 1)
	s.heavy = lim
	// schedule with short timeout; Acquire blocks until parent cancel via detachTimeout...
	// detachTimeout uses Background, so cancel parent won't help. Cancel via Shutdown gen.
	s.ScheduleEnrichOnly(context.Background(), 50*time.Millisecond)
	time.Sleep(80 * time.Millisecond)
	lim.Release()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

func TestShutdownTimeout(t *testing.T) {
	store := &stubStore{}
	s := New(stubGeo{}, store, 1)
	// hold workMu so Shutdown times out waiting
	s.workMu.Lock()
	s.ScheduleEnrichOnly(context.Background(), time.Minute)
	time.Sleep(20 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	s.Shutdown(ctx)
	s.workMu.Unlock()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second)
	defer cancel2()
	s.Shutdown(ctx2)
}
