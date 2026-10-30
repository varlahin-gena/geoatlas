package anomalyjob

import (
	"context"
	"testing"
	"time"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/adapter/heavytask"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

type stubStore struct{}

func (stubStore) Insert(context.Context, []usecaseanomaly.Event) error { return nil }
func (stubStore) List(context.Context, usecaseanomaly.ListQuery) ([]usecaseanomaly.Event, error) {
	return nil, nil
}
func (stubStore) ExistingFingerprints(context.Context, []string) (map[string]struct{}, error) {
	return nil, nil
}
func (stubStore) ActiveSuppressions(context.Context, []usecaseanomaly.SuppressionKey, time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	return nil, nil
}
func (stubStore) RecentSuppressionKeys(context.Context, string, []usecaseanomaly.SuppressionKey, time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	return nil, nil
}
func (stubStore) Ack(context.Context, string, string, time.Duration) error    { return nil }
func (stubStore) Assign(context.Context, string, string, string) error        { return nil }
func (stubStore) AssignIfEmpty(context.Context, string, string, string) error { return nil }
func (stubStore) CountSummary(context.Context, time.Time) (usecaseanomaly.Summary, error) {
	return usecaseanomaly.Summary{}, nil
}

type stubScan struct{}

func (stubScan) OldestLogTime(context.Context) (time.Time, error) { return time.Time{}, nil }
func (stubScan) PortScan(context.Context, time.Duration, int, int, bool, []usecaseanomaly.IPRange) ([]usecaseanomaly.PortScanHit, error) {
	return nil, nil
}
func (stubScan) HorizontalScan(context.Context, time.Duration, int, int, bool, []usecaseanomaly.IPRange) ([]usecaseanomaly.HorizontalScanHit, error) {
	return nil, nil
}
func (stubScan) BlockedCount(context.Context, time.Time, time.Time, *usecaseanomaly.IPRange) (uint64, error) {
	return 0, nil
}
func (stubScan) CurrentCountries(context.Context, time.Duration, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.CountryCount, error) {
	return nil, nil
}
func (stubScan) CurrentCountryTotal(context.Context, time.Duration, []usecaseanomaly.IPRange) (uint64, error) {
	return 0, nil
}
func (stubScan) BaselineCountries(context.Context, int, uint64, []usecaseanomaly.IPRange) (map[string]struct{}, error) {
	return nil, nil
}
func (stubScan) RecentEdges(context.Context, time.Duration, int, []usecaseanomaly.IPRange) ([]usecaseanomaly.EdgeRow, error) {
	return nil, nil
}
func (stubScan) KnownPairs(context.Context, [][2]string, time.Duration) (map[string]struct{}, error) {
	return nil, nil
}
func (stubScan) ByteSurge(context.Context, time.Duration, uint64, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.ByteSurgeHit, error) {
	return nil, nil
}
func (stubScan) Beaconing(context.Context, time.Duration, int, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.BeaconingHit, error) {
	return nil, nil
}
func (stubScan) LateralFanout(context.Context, time.Duration, int, int, []usecaseanomaly.IPRange) ([]usecaseanomaly.LateralFanoutHit, error) {
	return nil, nil
}

func TestNewClampsAndNilGuards(t *testing.T) {
	s := New(nil, time.Second, -time.Second)
	if s.interval < time.Minute || s.delay != time.Minute {
		t.Fatalf("interval=%v delay=%v", s.interval, s.delay)
	}
	var nilS *Scheduler
	nilS.SetLimiter(heavytask.New(1))
	nilS.SetInterval(time.Hour)
	nilS.Start(context.Background())
	nilS.Shutdown(context.Background())

	s.SetLimiter(nil)
	s.SetInterval(time.Second) // clamps to minute
	if s.interval < time.Minute {
		t.Fatal(s.interval)
	}
}

func TestStartUnavailableClosesDone(t *testing.T) {
	s := New(nil, time.Minute, 0)
	s.Start(context.Background())
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("done not closed")
	}
	s.Shutdown(context.Background())
}

func TestStartTickAndShutdown(t *testing.T) {
	svc := usecaseanomaly.New(usecaseanomaly.Config{Enabled: false}, stubStore{}, stubScan{}, nil, nil, nil)
	s := New(svc, time.Minute, 0)
	s.SetLimiter(heavytask.New(1))
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(30 * time.Millisecond)
	s.SetInterval(time.Minute)
	cancel()
	shut, shutCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutCancel()
	s.Shutdown(shut)
}

func TestTickBusyAndHeavySkip(t *testing.T) {
	svc := usecaseanomaly.New(usecaseanomaly.Config{Enabled: false}, stubStore{}, stubScan{}, nil, nil, nil)
	s := New(svc, time.Minute, 0)
	lim := heavytask.New(1)
	if !lim.TryAcquire() {
		t.Fatal("acquire")
	}
	s.heavy = lim
	s.tick(context.Background()) // heavy busy → skip
	lim.Release()

	s.busy.Lock()
	s.tick(context.Background()) // busy → skip
	s.busy.Unlock()

	s.tick(context.Background()) // runs Scan (disabled)
}

func TestGateSkipReason(t *testing.T) {
	g := Gate{}
	if g.SkipReason() != "" {
		t.Fatal(g.SkipReason())
	}
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "running"})
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
	})
	if g.SkipReason() != "rebuild" {
		t.Fatalf("%q", g.SkipReason())
	}
}
