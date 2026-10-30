package trafficstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"geoatlas/internal/adapter/clickhouse/query"
	"geoatlas/internal/model"
	"geoatlas/internal/usecase/events"
)

func TestComposeMapAggGeoPathWinsEvenIfEmpty(t *testing.T) {
	got := composeMapAgg("city", nil, true, []model.RawAgg{{SrcIP: "1.1.1.1"}})
	if got.Source != "geo_city" {
		t.Fatalf("source: %s", got.Source)
	}
	if len(got.Raws) != 0 {
		t.Fatalf("raw fallback must not run after geo path, raws=%d", len(got.Raws))
	}
	if got.GeoEdges != nil {
		t.Fatalf("empty geo rows should stay nil, got %#v", got.GeoEdges)
	}
}

func TestComposeMapAggRawWhenGeoUnavailable(t *testing.T) {
	raws := []model.RawAgg{{SrcIP: "10.0.0.1"}}
	got := composeMapAgg("ip", []model.GeoEdgeAgg{{SrcKey: "a"}}, false, raws)
	if got.Source != "ip_live_ip" {
		t.Fatalf("source: %s", got.Source)
	}
	if len(got.Raws) != 1 || got.Raws[0].SrcIP != "10.0.0.1" {
		t.Fatalf("raws: %#v", got.Raws)
	}
	if len(got.GeoEdges) != 0 {
		t.Fatalf("geo edges must be omitted on raw path: %#v", got.GeoEdges)
	}
}

func TestScopeCtxBackupUsesShadowTables(t *testing.T) {
	ctx := scopeCtx(context.Background(), "backup")
	tables := query.TablesOf(ctx)
	if !tables.IsBackup() {
		t.Fatalf("expected backup tables, got %#v", tables)
	}
	live := scopeCtx(context.Background(), "live")
	if query.TablesOf(live).IsBackup() {
		t.Fatal("live must not use backup tables")
	}
	empty := scopeCtx(context.Background(), "")
	if query.TablesOf(empty).IsBackup() {
		t.Fatal("empty dataSource must stay live")
	}
}

func TestNewTrafficRepository(t *testing.T) {
	ch := &stubConn{}
	r := NewTrafficRepository(ch)
	if r == nil || r.apiCH == nil {
		t.Fatal("NewTrafficRepository")
	}
}

func TestScanMapAggsGeoPath(t *testing.T) {
	ch := &stubConn{}
	ch.push(geoEdgeVals())
	r := NewTrafficRepository(ch)
	got, err := r.ScanMapAggs(context.Background(), model.TimeRange{Mode: "minutes", Amount: 30}, events.MapScanQuery{
		GroupBy: "city", Limit: 10, Filter: "all",
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "geo_city" || len(got.GeoEdges) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestScanMapAggsGeoErrorFallsBackToRaw(t *testing.T) {
	ch := &stubConn{}
	ch.pushErr(errors.New("geo down"))
	ch.push(rawAggVals())
	r := NewTrafficRepository(ch)
	got, err := r.ScanMapAggs(context.Background(), model.TimeRange{Mode: "minutes", Amount: 15}, events.MapScanQuery{
		GroupBy: "country", Limit: 5, Filter: "all",
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "ip_live_country" || len(got.Raws) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestScanMapAggsUnknownGroupByUsesRaw(t *testing.T) {
	ch := &stubConn{}
	ch.push(rawAggVals())
	r := NewTrafficRepository(ch)
	got, err := r.ScanMapAggs(context.Background(), model.TimeRange{Mode: "hours", Amount: 2}, events.MapScanQuery{
		GroupBy: "nope", Limit: 5, DataSource: "BACKUP",
	}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "ip_live_nope" || len(got.Raws) != 1 {
		t.Fatalf("%+v", got)
	}
}

func TestScanMapAggsRawError(t *testing.T) {
	ch := &stubConn{}
	ch.pushErr(errors.New("geo down"))
	ch.pushErr(errors.New("raw down"))
	r := NewTrafficRepository(ch)
	_, err := r.ScanMapAggs(context.Background(), model.TimeRange{Mode: "minutes", Amount: 10}, events.MapScanQuery{
		GroupBy: "city", Limit: 5,
	}, time.Second)
	if err == nil {
		t.Fatal("want raw error")
	}
}

func TestScanGeoMissingIPsForTimeRange(t *testing.T) {
	ch := &stubConn{}
	ch.push(geoMissingVals())
	r := NewTrafficRepository(ch)
	rows, err := r.ScanGeoMissingIPsForTimeRange(context.Background(), model.TimeRange{Mode: "hours", Amount: 6}, 100, time.Second)
	if err != nil || len(rows) != 1 || rows[0].IP != "9.9.9.9" {
		t.Fatalf("%v %v", rows, err)
	}
}

func TestScanCountrySeries(t *testing.T) {
	ch := &stubConn{}
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	ch.push(seriesVals(ts))
	r := NewTrafficRepository(ch)
	pts, bucket, err := r.ScanCountrySeries(context.Background(), model.TimeRange{Mode: "hours", Amount: 6}, "RU", "live", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if bucket <= 0 || len(pts) != 1 || pts[0].Total != 3 {
		t.Fatalf("pts=%+v bucket=%d", pts, bucket)
	}
}

func TestScanCountrySeriesEmptyCountry(t *testing.T) {
	r := NewTrafficRepository(&stubConn{})
	_, _, err := r.ScanCountrySeries(context.Background(), model.TimeRange{Mode: "days", Amount: 1}, "  ", "", time.Second)
	if err == nil {
		t.Fatal("want country required")
	}
}
