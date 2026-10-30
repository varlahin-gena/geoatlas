package query

import (
	"context"
	"errors"
	"testing"
	"time"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/model"
)

func TestTablesHelpers(t *testing.T) {
	live := LiveTables()
	if live.IsBackup() {
		t.Fatal("live")
	}
	bak := BackupTables()
	if !bak.IsBackup() {
		t.Fatal("backup")
	}
	if bak.GeoEdges("city") == "" || bak.GeoEdges("country") == "" || bak.GeoEdges("continent") == "" {
		t.Fatal("geo edges")
	}
	if bak.GeoEdges("ip") != "" {
		t.Fatal("ip has no geo table")
	}
	ctx := WithTables(context.Background(), bak)
	if TablesOf(ctx).Logs != bak.Logs {
		t.Fatal("TablesOf")
	}
	if TablesOf(nil).Logs != live.Logs {
		t.Fatal("nil ctx")
	}
	if TablesOf(context.Background()).Logs != live.Logs {
		t.Fatal("empty ctx")
	}
}

func TestPromoteTimeRange(t *testing.T) {
	tr := model.TimeRange{Mode: "hours", Amount: 48}
	got := promoteHoursToDays(tr, true)
	if got.Mode != "days" || got.Amount != 2 {
		t.Fatalf("%+v", got)
	}
	if promoteHoursToDays(tr, false).Mode != "hours" {
		t.Fatal("no prefer")
	}
	tr2 := model.TimeRange{Mode: "minutes", Amount: 120}
	got = promoteMinutesToHours(tr2, true)
	if got.Mode != "hours" || got.Amount != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestMapSelectScope(t *testing.T) {
	s := MapSelect{Country: "RU", Query: "x"}
	if s.scope().Country != "RU" {
		t.Fatal(s.scope())
	}
}

func TestPickSeriesBucketSeconds(t *testing.T) {
	cases := []struct {
		tr   model.TimeRange
		want int
	}{
		{model.TimeRange{Mode: "minutes", Amount: 30}, 5 * 60},
		{model.TimeRange{Mode: "hours", Amount: 6}, 15 * 60},
		{model.TimeRange{Mode: "hours", Amount: 48}, 60 * 60},
		{model.TimeRange{Mode: "days", Amount: 7}, 6 * 60 * 60},
		{model.TimeRange{Mode: "days", Amount: 30}, 24 * 60 * 60},
		{model.TimeRange{Mode: "absolute", From: time.Unix(0, 0), To: time.Unix(3600, 0)}, 5 * 60},
		{model.TimeRange{Mode: "weird"}, 60 * 60}, // default dur=24h → hourly buckets
	}
	for _, tc := range cases {
		if got := pickSeriesBucketSeconds(tc.tr); got != tc.want {
			t.Fatalf("%+v: got %d want %d", tc.tr, got, tc.want)
		}
	}
}

func TestScanRawAggRows(t *testing.T) {
	rows := &stubRows{rows: [][]any{rawAggVals(), {"bad"}}} // second row wrong arity → skip
	out, err := scanRawAggRows(rows)
	if err != nil || len(out) != 1 {
		t.Fatalf("%d %v", len(out), err)
	}
}

func TestScanGeoEdgeRows(t *testing.T) {
	rows := &stubRows{rows: [][]any{geoEdgeVals()}}
	out, err := scanGeoEdgeRows(rows)
	if err != nil || len(out) != 1 {
		t.Fatalf("%d %v", len(out), err)
	}
	if out[0].SrcLat != 55.0 || out[0].DstLon != -74.0 {
		t.Fatalf("coords %+v", out[0])
	}
}

func TestScanRawAggsForTimeRange(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	sel := MapSelect{Limit: 10, Filter: "all"}
	to := time.Second

	ch.push(rawAggVals())
	if rows, err := ScanRawAggsForTimeRange(ctx, ch, model.TimeRange{Mode: "minutes", Amount: 15}, sel, to); err != nil || len(rows) != 1 {
		t.Fatalf("%v %v", rows, err)
	}
	ch.push(rawAggVals())
	if _, err := ScanRawAggsForTimeRange(ctx, ch, model.TimeRange{Mode: "hours", Amount: 2}, sel, to); err != nil {
		t.Fatal(err)
	}

	// days without prefer → logs
	ch.push(rawAggVals())
	if _, err := ScanRawAggsForTimeRange(ctx, ch, model.TimeRange{Mode: "days", Amount: 3}, sel, to); err != nil {
		t.Fatal(err)
	}

	// days with prefer daily + success
	s := &aggstate.Store{}
	s.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "ready"})
	ctxAgg := aggstate.WithAgg(ctx, s)
	ch.push(rawAggVals())
	if _, err := ScanRawAggsForTimeRange(ctxAgg, ch, model.TimeRange{Mode: "days", Amount: 0}, sel, to); err != nil {
		t.Fatal(err)
	}
	// prefer + fail → fallback logs
	ch.pushErr(errors.New("daily down"))
	ch.push(rawAggVals())
	if _, err := ScanRawAggsForTimeRange(ctxAgg, ch, model.TimeRange{Mode: "days", Amount: 2}, sel, to); err != nil {
		t.Fatal(err)
	}

	from := time.Now().UTC().Add(-time.Hour)
	toT := time.Now().UTC()
	ch.push(rawAggVals())
	if _, err := ScanRawAggsForTimeRange(ctx, ch, model.TimeRange{Mode: "absolute", From: from, To: toT}, sel, to); err != nil {
		t.Fatal(err)
	}
	if rows, err := ScanRawAggsForTimeRange(ctx, ch, model.TimeRange{Mode: "absolute", From: toT, To: from}, sel, to); err != nil || rows != nil {
		t.Fatalf("%v %v", rows, err)
	}
	ch.push(rawAggVals())
	if _, err := ScanRawAggsForTimeRange(ctx, ch, model.TimeRange{Mode: "other"}, sel, to); err != nil {
		t.Fatal(err)
	}
}

func TestScanGeoEdgesForTimeRange(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	sel := MapSelect{Limit: 5}
	to := time.Second

	ok, used, err := ScanGeoEdgesForTimeRange(ctx, ch, model.TimeRange{Mode: "days", Amount: 1}, "nope", sel, to)
	if ok != nil || used || err != nil {
		t.Fatalf("%v %v %v", ok, used, err)
	}

	// city days without prefer → logs
	ch.push(geoEdgeVals())
	rows, used, err := ScanGeoEdgesForTimeRange(ctx, ch, model.TimeRange{Mode: "days", Amount: 1}, "city", sel, to)
	if err != nil || !used || len(rows) != 1 {
		t.Fatalf("%v used=%v err=%v", rows, used, err)
	}

	s := &aggstate.Store{}
	s.SetGeoEdgesAggReady(true)
	ctxGeo := aggstate.WithAgg(ctx, s)
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxGeo, ch, model.TimeRange{Mode: "days", Amount: 1}, "country", sel, to); err != nil || !used {
		t.Fatalf("used=%v err=%v", used, err)
	}
	ch.pushErr(errors.New("geo down"))
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxGeo, ch, model.TimeRange{Mode: "days", Amount: 1}, "continent", sel, to); err != nil || !used {
		t.Fatalf("fallback used=%v err=%v", used, err)
	}

	// ip days → logs (no daily prefer)
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctx, ch, model.TimeRange{Mode: "days", Amount: 1}, "ip", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}

	s2 := &aggstate.Store{}
	s2.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "ready"})
	ctxDaily := aggstate.WithAgg(ctx, s2)
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxDaily, ch, model.TimeRange{Mode: "days", Amount: 1}, "subnet", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}
	ch.pushErr(errors.New("ip daily down"))
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxDaily, ch, model.TimeRange{Mode: "days", Amount: 1}, "ip", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}

	// hours with hourly prefer
	s3 := &aggstate.Store{}
	s3.SetHourlyEdgesAggReady(true)
	ctxH := aggstate.WithAgg(ctx, s3)
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxH, ch, model.TimeRange{Mode: "hours", Amount: 3}, "city", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}
	ch.pushErr(errors.New("hourly down"))
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxH, ch, model.TimeRange{Mode: "hours", Amount: 2}, "ip", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctx, ch, model.TimeRange{Mode: "minutes", Amount: 30}, "country", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}

	from := time.Now().UTC().Add(-2 * time.Hour)
	toT := time.Now().UTC()
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxH, ch, model.TimeRange{Mode: "absolute", From: from, To: toT}, "city", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}
	ch.pushErr(errors.New("abs hourly"))
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxH, ch, model.TimeRange{Mode: "absolute", From: from, To: toT}, "ip", sel, to); err != nil || !used {
		t.Fatalf("%v %v", used, err)
	}
	if _, used, err := ScanGeoEdgesForTimeRange(ctx, ch, model.TimeRange{Mode: "absolute", From: toT, To: from}, "city", sel, to); used || err != nil {
		t.Fatalf("invalid abs used=%v err=%v", used, err)
	}
	if _, used, err := ScanGeoEdgesForTimeRange(ctx, ch, model.TimeRange{Mode: "weird"}, "city", sel, to); used || err != nil {
		t.Fatalf("weird used=%v err=%v", used, err)
	}

	// backup tables force prefer paths
	ctxBak := WithTables(ctx, BackupTables())
	ch.push(geoEdgeVals())
	if _, used, err := ScanGeoEdgesForTimeRange(ctxBak, ch, model.TimeRange{Mode: "days", Amount: 1}, "city", sel, to); err != nil || !used {
		t.Fatalf("bak %v %v", used, err)
	}
}

func TestScanCountrySeries(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	to := time.Second
	now := time.Now().UTC()
	if _, _, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "days", Amount: 1}, "", to); err == nil {
		t.Fatal("empty country")
	}
	ch.push([]any{now, uint64(1), uint64(2), uint64(3)})
	pts, bucket, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "minutes", Amount: 30}, "RU", to)
	if err != nil || len(pts) != 1 || bucket <= 0 {
		t.Fatalf("%v %d %v", pts, bucket, err)
	}
	ch.push([]any{now, uint64(1), uint64(2), uint64(3)})
	if _, _, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "hours", Amount: 2}, "RU", to); err != nil {
		t.Fatal(err)
	}
	ch.push([]any{now, uint64(1), uint64(2), uint64(3)})
	if _, _, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "days", Amount: 2}, "RU", to); err != nil {
		t.Fatal(err)
	}
	from := now.Add(-time.Hour)
	ch.push([]any{now, uint64(1), uint64(2), uint64(3)})
	if _, _, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "absolute", From: from, To: now}, "RU", to); err != nil {
		t.Fatal(err)
	}
	if pts, _, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "absolute", From: now, To: from}, "RU", to); err != nil || pts != nil {
		t.Fatalf("%v %v", pts, err)
	}
	ch.push([]any{now, uint64(1), uint64(2), uint64(3)})
	if _, _, err := ScanCountrySeries(ctx, ch, model.TimeRange{Mode: "x"}, "RU", to); err != nil {
		t.Fatal(err)
	}
}

func TestScanGeoMissingIPs(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	to := time.Second
	row := []any{"10.0.0.1", uint64(5), uint64(3), uint64(2), "1.2.3.4", "RU", "Msk", 0.0, 0.0}
	ch.push(row)
	out, err := ScanGeoMissingIPsForTimeRange(ctx, ch, model.TimeRange{Mode: "minutes", Amount: 10}, 0, to)
	if err != nil || len(out) != 1 {
		t.Fatalf("%v %v", out, err)
	}
	ch.push(row)
	if _, err := ScanGeoMissingIPsForTimeRange(ctx, ch, model.TimeRange{Mode: "hours", Amount: 1}, 9000, to); err != nil {
		t.Fatal(err)
	}
	ch.push(row)
	if _, err := ScanGeoMissingIPsForTimeRange(ctx, ch, model.TimeRange{Mode: "days", Amount: 1}, 10, to); err != nil {
		t.Fatal(err)
	}
	from := time.Now().UTC().Add(-time.Hour)
	toT := time.Now().UTC()
	ch.push(row)
	if _, err := ScanGeoMissingIPsForTimeRange(ctx, ch, model.TimeRange{Mode: "absolute", From: from, To: toT}, 10, to); err != nil {
		t.Fatal(err)
	}
	if out, err := ScanGeoMissingIPsForTimeRange(ctx, ch, model.TimeRange{Mode: "absolute", From: toT, To: from}, 10, to); err != nil || out != nil {
		t.Fatalf("%v %v", out, err)
	}
	ch.push(row)
	if _, err := ScanGeoMissingIPsForTimeRange(ctx, ch, model.TimeRange{Mode: "x"}, 10, to); err != nil {
		t.Fatal(err)
	}
}

func TestScanIPEdgesRelativeAbsolute(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	sel := MapSelect{Limit: 3}
	to := time.Second
	ch.push(geoEdgeVals())
	if rows, err := scanIPEdgesRelative(ctx, ch, "traffic_edges_daily", "day", "DAY", 0, "ip", sel, to); err != nil || len(rows) != 1 {
		t.Fatalf("%v %v", rows, err)
	}
	from := time.Now().UTC().Add(-time.Hour)
	toT := time.Now().UTC()
	ch.push(geoEdgeVals())
	if _, err := scanIPEdgesAbsolute(ctx, ch, "traffic_edges_hourly", "hour", from, toT, "city", sel, to); err != nil {
		t.Fatal(err)
	}
}

func TestScanRawLogsRelativeBadMode(t *testing.T) {
	_, err := scanRawLogsRelative(context.Background(), &stubConn{}, "weeks", 1, MapSelect{}, time.Second)
	if err == nil {
		t.Fatal("want error")
	}
	_, err = scanGeoFromLogsRelative(context.Background(), &stubConn{}, "city", "weeks", 1, MapSelect{}, time.Second)
	if err == nil {
		t.Fatal("want error")
	}
}
