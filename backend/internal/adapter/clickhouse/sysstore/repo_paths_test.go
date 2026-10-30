package sysstore

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/model"
)

func TestSystemRepositoryPaths(t *testing.T) {
	ch := &stubConn{}
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	ch.pushQuery(metricLatestVals(ts, 1.5), metricLatestVals(ts, math.NaN()), metricLatestVals(ts, math.Inf(1)))
	ch.pushQuery(metricHistVals(ts, 2.0), metricHistVals(ts, math.NaN()))
	ch.pushRow(uint64(42))
	ch.pingErr = nil

	r := NewSystemRepository(ch)
	latest, err := r.FetchLatest(context.Background())
	if err != nil || len(latest) != 3 {
		t.Fatalf("latest=%v err=%v", latest, err)
	}
	if latest[1].Value != 0 || latest[2].Value != 0 {
		t.Fatalf("NaN/Inf must zero: %+v", latest)
	}

	hist, err := r.FetchHistory(context.Background(), []model.MetricKey{{Type: "cpu", Target: "host", Name: "usage"}}, time.Hour, 0)
	if err != nil {
		t.Fatal(err)
	}
	key := model.MetricKey{Type: "cpu", Target: "host", Name: "usage"}.String()
	if len(hist[key]) != 2 || hist[key][1].Value != 0 {
		t.Fatalf("%+v", hist)
	}

	empty, err := r.FetchHistory(context.Background(), nil, time.Hour, time.Minute)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty keys: %v %v", empty, err)
	}

	n, err := r.CountRows(context.Background(), "traffic_logs")
	if err != nil || n != 42 {
		t.Fatalf("count=%d err=%v", n, err)
	}
	if err := r.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}

	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "ready", Phase: "backfill", Message: "ok", RawRows: 1, AggRows: 2})
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
		aggstate.SetGeoEdgesAggReady(false)
	})
	aggstate.SetGeoEdgesAggReady(true)
	st := r.Status()
	if st.State != "ready" || st.RawRows != 1 {
		t.Fatalf("%+v", st)
	}
	if !r.PreferDaily() || !r.PreferGeo() {
		t.Fatal("prefer flags")
	}
}

func TestFetchLatestAndHistoryErrors(t *testing.T) {
	ch := &stubConn{}
	ch.pushQueryErr(errors.New("qfail"))
	if _, err := FetchLatestMetrics(context.Background(), ch); err == nil {
		t.Fatal("want query err")
	}
	ch.pushQueryErr(errors.New("hfail"))
	if _, err := FetchMetricHistory(context.Background(), ch, []model.MetricKey{{Type: "a", Target: "b", Name: "c"}}, time.Minute, time.Second); err == nil {
		t.Fatal("want hist err")
	}

	ch.pushRowErr(errors.New("crow"))
	if _, err := CountTableRows(context.Background(), ch, "traffic_edges_daily"); err == nil {
		t.Fatal("want count err")
	}
}

func TestFetchLatestScanError(t *testing.T) {
	ch := &stubConn{}
	// wrong arity → Scan fails
	ch.pushQuery([]any{time.Now()})
	if _, err := FetchLatestMetrics(context.Background(), ch); err == nil {
		t.Fatal("want scan err")
	}
}
