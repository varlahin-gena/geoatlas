package anomalystore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

func TestNewRepository(t *testing.T) {
	ch := &stubConn{}
	r := New(ch)
	if r == nil || r.ch != ch {
		t.Fatal("New")
	}
}

func TestInsertPaths(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	r := New(ch)
	now := time.Now().UTC()
	err := r.Insert(ctx, []usecaseanomaly.Event{{
		DetectedAt: now, WindowStart: now, WindowEnd: now,
		Code: usecaseanomaly.CodePortScan, Severity: "high", Score: 1.5,
		Title: "t", Detail: map[string]any{"n": 1},
		Fingerprint: "fp1", SuppressionKey: "sk",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if ch.batchAppend != 1 {
		t.Fatalf("append=%d", ch.batchAppend)
	}
	// empty src/dst → zero IPv4
	ch2 := &stubConn{}
	if err := New(ch2).Insert(ctx, []usecaseanomaly.Event{{
		DetectedAt: now, Code: "x", Fingerprint: "fp2",
	}}); err != nil {
		t.Fatal(err)
	}
	ch3 := &stubConn{prepareErr: errors.New("prep")}
	if err := New(ch3).Insert(ctx, []usecaseanomaly.Event{{Fingerprint: "x"}}); err == nil {
		t.Fatal("want prepare err")
	}
	ch4 := &stubConn{appendErr: errors.New("append")}
	if err := New(ch4).Insert(ctx, []usecaseanomaly.Event{{Fingerprint: "x"}}); err == nil {
		t.Fatal("want append err")
	}
}

func TestListAndSummary(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	now := time.Now().UTC()
	ch.pushQuery([]any{
		now, now, now, usecaseanomaly.CodePortScan, "high", float32(2),
		"title", `{"k":1}`, "203.0.113.1", "0.0.0.0",
		"RU", "US", "Moscow", "NYC", "fw1",
		uint64(3), "fp", "ep", now, uint8(1), "admin", "ops",
	})
	r := New(ch)
	items, err := r.List(ctx, usecaseanomaly.ListQuery{
		Since: now.Add(-time.Hour), Severity: "high", Code: usecaseanomaly.CodePortScan,
		IncludeAcked: true, Limit: 10,
	})
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if items[0].SrcIP == "" || items[0].DstIP != "" {
		t.Fatalf("displayIP: src=%q dst=%q", items[0].SrcIP, items[0].DstIP)
	}
	if _, err := r.List(ctx, usecaseanomaly.ListQuery{Limit: 0}); err == nil {
		t.Fatal("invalid limit")
	}

	ch.pushRow(uint64(5), uint64(2), uint64(1), uint64(3))
	sum, err := r.CountSummary(ctx, now.Add(-time.Hour))
	if err != nil || sum.Total != 5 || sum.High != 2 {
		t.Fatalf("%+v err=%v", sum, err)
	}
}

func TestFingerprintsSuppressionsAckAssign(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	r := New(ch)

	ch.pushQuery([]any{"fp1"}, []any{"fp2"})
	m, err := r.ExistingFingerprints(ctx, []string{"fp1", "fp2", "fp3"})
	if err != nil || len(m) != 2 {
		t.Fatalf("m=%v err=%v", m, err)
	}

	ch.pushQuery([]any{"k1"})
	sm, err := r.ActiveSuppressions(ctx, []usecaseanomaly.SuppressionKey{"k1", ""}, time.Now())
	if err != nil || len(sm) != 1 {
		t.Fatalf("%v %v", sm, err)
	}
	if _, err := r.ActiveSuppressions(ctx, []usecaseanomaly.SuppressionKey{""}, time.Now()); err != nil {
		t.Fatal(err)
	}

	key := usecaseanomaly.SuppressionKey(usecaseanomaly.CodeNewCountryDst + "|country|CN")
	ch.pushQuery([]any{"CN"})
	rm, err := r.RecentSuppressionKeys(ctx, usecaseanomaly.CodeNewCountryDst, []usecaseanomaly.SuppressionKey{key, "bad"}, time.Now().Add(-time.Hour))
	if err != nil || len(rm) != 1 {
		t.Fatalf("%v %v", rm, err)
	}
	ch.pushQuery([]any{"sk"})
	rm, err = r.RecentSuppressionKeys(ctx, usecaseanomaly.CodePortScan, []usecaseanomaly.SuppressionKey{"sk", ""}, time.Now().Add(-time.Hour))
	if err != nil || len(rm) != 1 {
		t.Fatalf("%v %v", rm, err)
	}

	// Ack without suppress
	if err := r.Ack(ctx, "fp", "admin", 0); err != nil {
		t.Fatal(err)
	}
	// Ack with suppress — compute key from fields
	ch.pushRow(usecaseanomaly.CodePortScan, "", "1.2.3.4", "5.6.7.8", "US", "fw")
	if err := r.Ack(ctx, "fp", "admin", time.Hour); err != nil {
		t.Fatal(err)
	}
	ch.pushRow(usecaseanomaly.CodeNewCountryDst, "", "", "", "DE", "")
	if err := r.Ack(ctx, "fp", "admin", time.Hour); err != nil {
		t.Fatal(err)
	}
	ch.pushRow(usecaseanomaly.CodeRepNewDst, "", "1.1.1.1", "2.2.2.2", "", "")
	if err := r.Ack(ctx, "fp", "admin", time.Hour); err != nil {
		t.Fatal(err)
	}
	ch.pushRow(usecaseanomaly.CodeBlockedSurge, "", "", "", "", "net1")
	if err := r.Ack(ctx, "fp", "admin", time.Hour); err != nil {
		t.Fatal(err)
	}
	ch.pushRow(usecaseanomaly.CodeBlockedSurge, "", "", "", "", "")
	if err := r.Ack(ctx, "fp", "admin", time.Hour); err != nil {
		t.Fatal(err)
	}
	ch.pushRow("other", "prefilled", "", "", "", "")
	if err := r.Ack(ctx, "fp", "admin", time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := r.Assign(ctx, "fp", "ops", "admin"); err != nil {
		t.Fatal(err)
	}
	ch.pushRow("already")
	if err := r.AssignIfEmpty(ctx, "fp", "ops", "admin"); err != nil {
		t.Fatal(err)
	}
	ch.pushRowErr(errors.New("no rows"))
	if err := r.AssignIfEmpty(ctx, "fp", "ops", "admin"); err != nil {
		t.Fatal(err)
	}
}

func TestTrafficScansHappyPaths(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	r := New(ch)
	nets := []usecaseanomaly.IPRange{{Start: 10, End: 20}}
	now := time.Now().UTC()

	ch.pushRow(now)
	if ts, err := r.OldestLogTime(ctx); err != nil || ts.IsZero() {
		t.Fatalf("%v %v", ts, err)
	}

	ch.pushQuery([]any{"1.1.1.1", uint64(20), uint64(50), "RU"})
	if hits, err := r.PortScan(ctx, time.Hour, 10, 10, false, nets); err != nil || len(hits) != 1 {
		t.Fatalf("%v %v", hits, err)
	}
	ch.pushQuery([]any{"1.1.1.1", "1.1.1.0/24", uint64(30), uint64(40)})
	if hits, err := r.HorizontalScan(ctx, time.Hour, 10, 10, true, nil); err != nil || len(hits) != 1 {
		t.Fatalf("%v %v", hits, err)
	}

	ch.pushRow(uint64(9))
	net := nets[0]
	if n, err := r.BlockedCount(ctx, now.Add(-time.Hour), now, &net); err != nil || n != 9 {
		t.Fatalf("%d %v", n, err)
	}
	ch.pushRow(uint64(1))
	if _, err := r.BlockedCount(ctx, now.Add(-time.Hour), now, nil); err != nil {
		t.Fatal(err)
	}

	ch.pushQuery([]any{"CN", uint64(5)})
	if cc, err := r.CurrentCountries(ctx, time.Hour, 1, nets); err != nil || len(cc) != 1 {
		t.Fatalf("%v %v", cc, err)
	}
	ch.pushRow(uint64(100))
	if total, err := r.CurrentCountryTotal(ctx, time.Hour, nil); err != nil || total != 100 {
		t.Fatalf("%d %v", total, err)
	}

	ch.pushQuery([]any{"US"}, []any{""})
	if set, err := r.BaselineCountries(ctx, 7, 1, nets); err != nil || len(set) != 1 {
		t.Fatalf("%v %v", set, err)
	}
	// no nets: edges table ok
	ch.pushQuery([]any{"DE"})
	if set, err := r.BaselineCountries(ctx, 7, 1, nil); err != nil || len(set) != 1 {
		t.Fatalf("%v %v", set, err)
	}
	// edges fail → logs fallback
	ch.pushQueryErr(errors.New("no table"))
	ch.pushQuery([]any{"FR"})
	if set, err := r.BaselineCountries(ctx, 7, 1, nil); err != nil || len(set) != 1 {
		t.Fatalf("%v %v", set, err)
	}

	ch.pushQuery([]any{"1.1.1.1", "2.2.2.2", uint64(3), "RU", "US"})
	if edges, err := r.RecentEdges(ctx, time.Hour, 0, nil); err != nil || len(edges) != 1 {
		t.Fatalf("%v %v", edges, err)
	}
	ch.pushQueryErr(errors.New("missing"))
	ch.pushQuery([]any{"1.1.1.1", "2.2.2.2", uint64(1), "RU", "US"})
	if _, err := r.RecentEdges(ctx, time.Hour, 5, nets); err != nil {
		t.Fatal(err)
	}

	ch.pushQuery([]any{"1.1.1.1", "2.2.2.2"})
	if pairs, err := r.KnownPairs(ctx, [][2]string{{"1.1.1.1", "2.2.2.2"}, {"", "x"}}, time.Hour); err != nil || len(pairs) != 1 {
		t.Fatalf("%v %v", pairs, err)
	}
	ch.pushQueryErr(errors.New("missing"))
	ch.pushQuery([]any{"a", "b"})
	if _, err := r.KnownPairs(ctx, [][2]string{{"a", "b"}}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if m, err := r.KnownPairs(ctx, [][2]string{{"", ""}}, time.Hour); err != nil || len(m) != 0 {
		t.Fatalf("%v %v", m, err)
	}

	ch.pushQuery([]any{"1.1.1.1", uint64(1000), uint64(10)})
	if hits, err := r.ByteSurge(ctx, time.Second, 1, 100, nil); err != nil || len(hits) != 1 {
		t.Fatalf("%v %v", hits, err)
	}
	ch.pushQueryErr(errors.New("missing"))
	ch.pushQuery([]any{"1.1.1.1", uint64(1000), uint64(10)})
	if _, err := r.ByteSurge(ctx, time.Hour, 1, 100, nets); err != nil {
		t.Fatal(err)
	}

	ch.pushQuery([]any{"1.1.1.1", uint64(8), uint64(20)})
	if hits, err := r.LateralFanout(ctx, time.Hour, 5, 5, nets); err != nil || len(hits) != 1 {
		t.Fatalf("%v %v", hits, err)
	}
}

func TestBeaconingPaths(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	r := New(ch)

	// hourly ok + hours attach
	ch.pushQuery([]any{"1.1.1.1", "2.2.2.2", uint64(5), uint64(100), uint64(10)})
	ch.pushQuery([]any{"1.1.1.1", "2.2.2.2", []int64{1, 2, 3}})
	hits, err := r.Beaconing(ctx, time.Minute, 1, 1000, nil)
	if err != nil || len(hits) != 1 || len(hits[0].HourUnix) != 3 {
		t.Fatalf("%+v err=%v", hits, err)
	}

	// hourly UNKNOWN_TABLE → logs
	ch.pushQueryErr(&clickhouse.Exception{Code: 60, Message: "UNKNOWN_TABLE"})
	ch.pushQuery([]any{"3.3.3.3", "4.4.4.4", uint64(3), uint64(30), uint64(5)})
	ch.pushQueryErr(errors.New("hours fail")) // attach returns candidates without hours
	hits, err = r.Beaconing(ctx, 24*time.Hour, 2, 1000, []usecaseanomaly.IPRange{{Start: 1, End: 2}})
	if err != nil || len(hits) != 1 {
		t.Fatalf("%v %v", hits, err)
	}

	// empty candidates
	ch.pushQuery() // empty rows
	hits, err = r.Beaconing(ctx, 24*time.Hour, 2, 1000, nil)
	if err != nil || len(hits) != 0 {
		t.Fatalf("%v %v", hits, err)
	}
}

func TestScanCountrySetNilOut(t *testing.T) {
	rows := &stubRows{rows: [][]any{{"XX"}, {""}}}
	out, err := scanCountrySet(rows, nil)
	if err != nil || len(out) != 1 {
		t.Fatalf("%v %v", out, err)
	}
}

func TestIPv4AsUInt32SQL(t *testing.T) {
	if got := ipv4AsUInt32SQL("src_ip"); got == "" {
		t.Fatal("empty")
	}
}
