package huntadapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"geoatlas/internal/model"
	usecaseanomaly "geoatlas/internal/usecase/anomaly"
	usecaseevents "geoatlas/internal/usecase/events"
	"geoatlas/internal/usecase/hunts"
)

func TestHeavyGate(t *testing.T) {
	g := HeavyGate{}
	if !g.TryAcquire() {
		t.Fatal("nil try defaults true")
	}
	g.Release()

	n := 0
	g = HeavyGate{
		Try: func() bool { n++; return false },
		Rel: func() { n += 10 },
	}
	if g.TryAcquire() {
		t.Fatal("want false")
	}
	g.Release()
	if n != 11 {
		t.Fatalf("n=%d", n)
	}
}

func TestMapRunnerNilEvents(t *testing.T) {
	r := MapRunner{}
	_, err := r.RunMap(context.Background(), hunts.MapState{Period: "1d"}, time.Second)
	if err == nil {
		t.Fatal("want unavailable")
	}
}

type stubTraffic struct {
	res usecaseevents.MapAggScanResult
	err error
}

func (s stubTraffic) ScanMapAggs(context.Context, model.TimeRange, usecaseevents.MapScanQuery, time.Duration) (usecaseevents.MapAggScanResult, error) {
	return s.res, s.err
}
func (s stubTraffic) ScanCountrySeries(context.Context, model.TimeRange, string, string, time.Duration) ([]usecaseevents.SeriesPoint, int, error) {
	return nil, 0, nil
}

func TestMapRunnerOK(t *testing.T) {
	repo := stubTraffic{res: usecaseevents.MapAggScanResult{
		Source: "geo_city",
		GeoEdges: []model.GeoEdgeAgg{{
			SrcKey: "A", DstKey: "B", SrcLabel: "A", DstLabel: "B",
			SrcLat: 1, SrcLon: 2, DstLat: 3, DstLon: 4,
			Count: 2, AllowedCnt: 2,
		}},
	}}
	r := MapRunner{Events: usecaseevents.New(repo, nil, nil)}
	snap, err := r.RunMap(context.Background(), hunts.MapState{
		Period: "6h", GroupBy: "city", Filter: "all", Limit: 100,
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if snap.EdgeCount < 1 || snap.RawPairs < 1 {
		t.Fatalf("%+v", snap)
	}
}

func TestMapRunnerGetMapError(t *testing.T) {
	repo := stubTraffic{err: errors.New("scan fail")}
	r := MapRunner{Events: usecaseevents.New(repo, nil, nil)}
	_, err := r.RunMap(context.Background(), hunts.MapState{Period: "1d", GroupBy: "ip"}, time.Second)
	if err == nil {
		t.Fatal("want err")
	}
}

type anomalyStore struct {
	inserted []usecaseanomaly.Event
}

func (a *anomalyStore) Insert(_ context.Context, events []usecaseanomaly.Event) error {
	a.inserted = append(a.inserted, events...)
	return nil
}
func (a *anomalyStore) List(context.Context, usecaseanomaly.ListQuery) ([]usecaseanomaly.Event, error) {
	return nil, nil
}
func (a *anomalyStore) ExistingFingerprints(context.Context, []string) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (a *anomalyStore) ActiveSuppressions(context.Context, []usecaseanomaly.SuppressionKey, time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	return map[usecaseanomaly.SuppressionKey]struct{}{}, nil
}
func (a *anomalyStore) RecentSuppressionKeys(context.Context, string, []usecaseanomaly.SuppressionKey, time.Time) (map[usecaseanomaly.SuppressionKey]struct{}, error) {
	return map[usecaseanomaly.SuppressionKey]struct{}{}, nil
}
func (a *anomalyStore) Ack(context.Context, string, string, time.Duration) error { return nil }
func (a *anomalyStore) Assign(context.Context, string, string, string) error     { return nil }
func (a *anomalyStore) AssignIfEmpty(context.Context, string, string, string) error {
	return nil
}
func (a *anomalyStore) CountSummary(context.Context, time.Time) (usecaseanomaly.Summary, error) {
	return usecaseanomaly.Summary{}, nil
}

type anomalyScan struct{}

func (anomalyScan) OldestLogTime(context.Context) (time.Time, error) { return time.Time{}, nil }
func (anomalyScan) PortScan(context.Context, time.Duration, int, int, bool, []usecaseanomaly.IPRange) ([]usecaseanomaly.PortScanHit, error) {
	return nil, nil
}
func (anomalyScan) HorizontalScan(context.Context, time.Duration, int, int, bool, []usecaseanomaly.IPRange) ([]usecaseanomaly.HorizontalScanHit, error) {
	return nil, nil
}
func (anomalyScan) BlockedCount(context.Context, time.Time, time.Time, *usecaseanomaly.IPRange) (uint64, error) {
	return 0, nil
}
func (anomalyScan) CurrentCountries(context.Context, time.Duration, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.CountryCount, error) {
	return nil, nil
}
func (anomalyScan) CurrentCountryTotal(context.Context, time.Duration, []usecaseanomaly.IPRange) (uint64, error) {
	return 0, nil
}
func (anomalyScan) BaselineCountries(context.Context, int, uint64, []usecaseanomaly.IPRange) (map[string]struct{}, error) {
	return nil, nil
}
func (anomalyScan) RecentEdges(context.Context, time.Duration, int, []usecaseanomaly.IPRange) ([]usecaseanomaly.EdgeRow, error) {
	return nil, nil
}
func (anomalyScan) KnownPairs(context.Context, [][2]string, time.Duration) (map[string]struct{}, error) {
	return nil, nil
}
func (anomalyScan) ByteSurge(context.Context, time.Duration, uint64, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.ByteSurgeHit, error) {
	return nil, nil
}
func (anomalyScan) Beaconing(context.Context, time.Duration, int, uint64, []usecaseanomaly.IPRange) ([]usecaseanomaly.BeaconingHit, error) {
	return nil, nil
}
func (anomalyScan) LateralFanout(context.Context, time.Duration, int, int, []usecaseanomaly.IPRange) ([]usecaseanomaly.LateralFanoutHit, error) {
	return nil, nil
}

func TestHuntAnomalyReporter(t *testing.T) {
	r := HuntAnomalyReporter{}
	if err := r.ReportHuntBreach(context.Background(), hunts.Hunt{Name: "h"}, hunts.RunResult{EdgeCount: 3}, time.Time{}); err != nil {
		t.Fatal(err)
	}

	store := &anomalyStore{}
	svc := usecaseanomaly.New(usecaseanomaly.Config{}, store, anomalyScan{}, nil, nil, nil)
	r = HuntAnomalyReporter{Anomaly: svc}
	now := time.Date(2024, 2, 3, 4, 0, 0, 0, time.UTC)
	if err := r.ReportHuntBreach(context.Background(), hunts.Hunt{
		ID: "id1", Name: "HuntA",
		Map: hunts.MapState{Period: "1d", GroupBy: "city", Filter: "all", Query: "q", Country: "RU"},
	}, hunts.RunResult{EdgeCount: 10, PrevEdges: 2, Ratio: 5, QueryCost: "medium"}, now); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 1 {
		t.Fatalf("%d", len(store.inserted))
	}
	if store.inserted[0].Code != usecaseanomaly.CodeHuntThreshold {
		t.Fatalf("%+v", store.inserted[0])
	}
	if err := r.ReportHuntBreach(context.Background(), hunts.Hunt{ID: "id2", Name: "B"}, hunts.RunResult{EdgeCount: 1}, time.Time{}); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 2 {
		t.Fatalf("%d", len(store.inserted))
	}
}
