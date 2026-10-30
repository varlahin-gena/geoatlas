package geostore

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
)

type stubResolver struct {
	n       int
	lookups map[string]model.GeoLookup
}

func (s stubResolver) RangeCount() int { return s.n }
func (s stubResolver) Lookup(ip string) model.GeoLookup {
	if s.lookups == nil {
		return model.GeoLookup{}
	}
	return s.lookups[ip]
}

func TestIsNoRows(t *testing.T) {
	if isNoRows(nil) || !isNoRows(sql.ErrNoRows) || !isNoRows(errors.New("no rows")) {
		t.Fatal("isNoRows")
	}
	if isNoRows(errors.New("connection refused")) {
		t.Fatal("other")
	}
}

func TestLoadCountStampFindList(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}

	ch.pushQuery(geoRangeVals(1, 10, "RU"), geoRangeVals(20, 30, "US"))
	got, err := LoadGeoRanges(ctx, ch)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}

	ch.pushQuery(geoRangeVals(1, 10, "RU"))
	snap, err := LoadGeoSnapshot(ctx, ch)
	if err != nil || snap == nil || snap.RangeCount() < 1 {
		t.Fatalf("%v %v", snap, err)
	}

	ch.pushRow(uint64(42))
	n, err := CountGeoRanges(ctx, ch)
	if err != nil || n != 42 {
		t.Fatalf("%d %v", n, err)
	}

	ch.pushRow(uint64(3), uint32(1), uint32(99), uint64(10), uint64(20), uint64(7))
	st, err := QuerySourceStamp(ctx, ch)
	if err != nil || st.Count != 3 || st.MinStart != 1 {
		t.Fatalf("%+v %v", st, err)
	}

	if _, ok, err := FindGeoRangeByIP(ctx, ch, "not-an-ip"); err != nil || ok {
		t.Fatalf("bad ip ok=%v err=%v", ok, err)
	}
	ch.pushRow(uint32(0x01010101), uint32(0x010101FF), "RU", "r", "c", 1.0, 2.0)
	g, ok, err := FindGeoRangeByIP(ctx, ch, "1.1.1.5")
	if err != nil || !ok || g.Country != "RU" {
		t.Fatalf("%+v ok=%v err=%v", g, ok, err)
	}
	ch.pushRowErr(sql.ErrNoRows)
	if _, ok, err := FindGeoRangeByIP(ctx, ch, "8.8.8.8"); err != nil || ok {
		t.Fatalf("no rows ok=%v err=%v", ok, err)
	}

	ch.pushRow(uint64(5)) // total
	ch.pushQuery(geoRangeVals(1, 2, "RU"))
	items, total, filtered, trunc, err := ListGeoRangesPage(ctx, ch, 0, "")
	if err != nil || len(items) != 1 || total != 5 || filtered != 5 || trunc {
		t.Fatalf("items=%d total=%d filt=%d trunc=%v err=%v", len(items), total, filtered, trunc, err)
	}

	ch.pushRow(uint64(10)) // total
	ch.pushRow(uint64(3))  // filtered count
	ch.pushQuery(geoRangeVals(1, 2, "RU"))
	items, total, filtered, trunc, err = ListGeoRangesPage(ctx, ch, 2, "ru")
	if err != nil || len(items) != 1 || total != 10 || filtered != 3 || !trunc {
		t.Fatalf("q items=%d total=%d filt=%d trunc=%v err=%v", len(items), total, filtered, trunc, err)
	}
}

func TestReplaceTruncateInsert(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	ranges := []model.GeoRange{{StartIP: 1, EndIP: 2, Country: "RU"}}

	// ReplaceViaStaging: DROP, CREATE AS, fill (PrepareBatch+Send), QueryRow count, EXCHANGE, DROP
	ch.pushRow(uint64(1)) // staging count
	n, err := ReplaceGeoRanges(ctx, ch, ranges)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if ch.batchAppend != 1 {
		t.Fatalf("append=%d", ch.batchAppend)
	}

	if err := TruncateGeoRanges(ctx, ch); err != nil {
		t.Fatal(err)
	}
	joined := false
	for _, q := range ch.execCalls {
		if q == "TRUNCATE TABLE IF EXISTS geo_ranges" {
			joined = true
		}
	}
	if !joined {
		t.Fatalf("truncate missing: %#v", ch.execCalls)
	}
}

func TestEnterpriseNets(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	now := time.Now().UTC()
	ch.pushQuery([]any{uint32(1), uint32(2), "10.0.0.0/8", "lab", "RU", "r", "c", now})
	nets, err := listEnterpriseNets(ctx, ch)
	if err != nil || len(nets) != 1 {
		t.Fatalf("%v %v", nets, err)
	}
	ch.pushRow(uint64(7))
	n, err := countEnterpriseNets(ctx, ch)
	if err != nil || n != 7 {
		t.Fatalf("%d %v", n, err)
	}
	if err := upsertEnterpriseNet(ctx, ch, model.EnterpriseNet{StartIP: 1, EndIP: 2, Network: "n", Label: "l"}); err != nil {
		t.Fatal(err)
	}
	if err := deleteEnterpriseNet(ctx, ch, 1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestGeoRepositoryWrappers(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	r := NewGeoRepository(nil, ch) // api falls back to write
	if r.apiConn() != ch {
		t.Fatal("apiConn fallback")
	}
	r2 := NewGeoRepository(ch, ch)
	if r2.apiConn() != ch {
		t.Fatal("apiConn")
	}

	ch.pushRow(uint64(1))
	if n, err := r.Count(ctx); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	ch.pushQuery(geoRangeVals(1, 2, "RU"))
	if items, err := r.Load(ctx); err != nil || len(items) != 1 {
		t.Fatalf("%v %v", items, err)
	}
	ch.pushRowErr(sql.ErrNoRows)
	if _, ok, err := r.FindByIP(ctx, "1.2.3.4"); err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	ch.pushRow(uint64(1))
	ch.pushQuery(geoRangeVals(1, 2, "RU"))
	if items, _, _, _, err := r.ListPage(ctx, 10, ""); err != nil || len(items) != 1 {
		t.Fatalf("%v %v", items, err)
	}
	ch.pushRow(uint64(1))
	if n, err := r.Replace(ctx, []model.GeoRange{{StartIP: 1, EndIP: 2}}); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	if err := r.Truncate(ctx); err != nil {
		t.Fatal(err)
	}

	ch.pushQuery([]any{uint32(1), uint32(2), "n", "l", "", "", "", time.Now().UTC()})
	if nets, err := r.ListEnterpriseNets(ctx); err != nil || len(nets) != 1 {
		t.Fatalf("%v %v", nets, err)
	}
	ch.pushRow(uint64(2))
	if n, err := r.CountEnterpriseNets(ctx); err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	if err := r.UpsertEnterpriseNet(ctx, model.EnterpriseNet{StartIP: 1, EndIP: 2}); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteEnterpriseNet(ctx, 1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestEnrichLogsMissingGeo(t *testing.T) {
	ctx := context.Background()
	if n, err := EnrichLogsMissingGeo(ctx, nil, stubResolver{n: 1}, 7); err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
	ch := &stubConn{}
	if n, err := EnrichLogsMissingGeo(ctx, ch, stubResolver{n: 0}, 7); err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}

	// table type IPv4 already ok
	ch.pushRow("IPv4")
	ch.pushQuery() // empty ips → nothing to enrich
	n, err := EnrichLogsMissingGeo(ctx, ch, stubResolver{n: 10}, 7)
	if err != nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}

	// String type → drop+create; then enrich
	ch2 := &stubConn{}
	ch2.pushRow("String")
	ch2.pushQuery([]any{"203.0.113.10"}, []any{"bad"}, []any{"203.0.113.11"})
	res := stubResolver{
		n: 5,
		lookups: map[string]model.GeoLookup{
			"203.0.113.10": {Found: true, Country: "RU", Lat: 55, Lon: 37},
			"203.0.113.11": {Found: true, Country: ""}, // no coords/country usable → skip
		},
	}
	n, err = EnrichLogsMissingGeo(ctx, ch2, res, 0)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}

	// QueryRow type missing → create path (err on type lookup)
	ch3 := &stubConn{}
	ch3.pushRowErr(errors.New("no rows"))
	ch3.pushQuery([]any{"198.51.100.1"})
	res2 := stubResolver{n: 1, lookups: map[string]model.GeoLookup{
		"198.51.100.1": {Found: true, Country: "US", City: "NYC", Lat: 40, Lon: -74},
	}}
	n, err = EnrichLogsMissingGeo(ctx, ch3, res2, 3)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestReloadableGeoIndexPaths(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, "geo.snap")
	ch := &stubConn{}

	idx := NewReloadableGeoIndex(ch, snap)
	if idx.IndexReady() {
		t.Fatal("not ready yet")
	}
	if idx.LoadDisk() {
		t.Fatal("missing disk")
	}

	ranges := []model.GeoRange{{StartIP: 1, EndIP: 10, Country: "RU", City: "Msk", Lat: 1, Lon: 2}}
	idx.ReplaceRanges(ranges)
	if !idx.IndexReady() || idx.RangeCount() < 1 {
		t.Fatal("replace ranges")
	}
	if _, err := os.Stat(snap); err != nil {
		t.Fatalf("persist: %v", err)
	}

	idx2 := NewReloadableGeoIndex(ch, snap)
	if !idx2.LoadDisk() || !idx2.IndexReady() {
		t.Fatal("load disk")
	}

	idx.ReplaceNormalizedRanges(ranges)
	b := geoip.NewCompactBuilder(len(ranges))
	for _, g := range ranges {
		b.AddRange(g)
	}
	idx.ReplaceBuiltSnapshot(b.Build())

	// Reload: stamp matches in-memory → early return
	st := geoip.StampFromRanges(ranges)
	ch.pushRow(st.Count, st.MinStart, st.MaxEnd, st.SumStart, st.SumEnd, st.XorSpan)
	if err := idx.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Reload: stamp differs → full LoadGeoSnapshot
	ch.pushRow(uint64(999), uint32(0), uint32(0), uint64(0), uint64(0), uint64(0))
	ch.pushQuery(geoRangeVals(1, 10, "RU"))
	ch.pushRow(uint64(1), uint32(1), uint32(10), uint64(1), uint64(10), uint64(1)) // persist stamp after load when first stamp "ok"
	// Actually after LoadGeoSnapshot, stampErr==nil so persist(chStamp) with the first stamp (999...).
	// No second QuerySourceStamp. Good.
	if err := idx.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	// Reload when stamp query fails → full reload + second stamp attempt
	ch.pushRowErr(errors.New("stamp fail"))
	ch.pushQuery(geoRangeVals(5, 6, "DE"))
	ch.pushRow(uint64(1), uint32(5), uint32(6), uint64(5), uint64(6), uint64(1))
	if err := idx.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}

	empty := NewReloadableGeoIndex(ch, "")
	empty.ReplaceRanges(nil) // clear / no persist path
}

func TestScanGeoRangeRowsError(t *testing.T) {
	rows := &stubRows{rows: [][]any{{"bad"}}}
	if _, err := scanGeoRangeRows(rows); err == nil {
		t.Fatal("want scan error")
	}
}

func TestInsertCanceledAndStampHelpers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := &stubConn{}
	if _, err := insertGeoRangesInto(ctx, ch, "geo_ranges", []model.GeoRange{{StartIP: 1, EndIP: 2}}); err == nil {
		t.Fatal("want canceled")
	}
	if st := stampFromBuilt(nil); st.Count != 0 {
		t.Fatal(st)
	}
	if _, _, err := readSnapshotFile(""); err == nil {
		t.Fatal("empty path")
	}
}

func TestReloadDiskStampMatch(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, "geo.snap")
	ch := &stubConn{}
	idx := NewReloadableGeoIndex(ch, snap)
	ranges := []model.GeoRange{{StartIP: 10, EndIP: 20, Country: "KZ"}}
	idx.ReplaceRanges(ranges)
	st := geoip.StampFromRanges(ranges)

	// Force in-memory stamp mismatch, disk matches CH → load from disk branch.
	idx.setStamp(geoip.SourceStamp{Count: 999})
	ch.pushRow(st.Count, st.MinStart, st.MaxEnd, st.SumStart, st.SumEnd, st.XorSpan)
	if err := idx.Reload(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestNilGuardsExtra(t *testing.T) {
	ctx := context.Background()
	if _, err := LoadGeoSnapshot(ctx, nil); err == nil {
		t.Fatal("nil snap")
	}
	if _, err := CountGeoRanges(ctx, nil); err == nil {
		t.Fatal("nil count")
	}
	if err := TruncateGeoRanges(ctx, nil); err == nil {
		t.Fatal("nil truncate")
	}
	if _, _, err := FindGeoRangeByIP(ctx, nil, "1.1.1.1"); err == nil {
		t.Fatal("nil find")
	}
	if _, _, _, _, err := ListGeoRangesPage(ctx, nil, 1, ""); err == nil {
		t.Fatal("nil list")
	}
	var nilIdx *ReloadableGeoIndex
	nilIdx.ReplaceBuiltSnapshot(nil)
	nilIdx.ReplaceNormalizedRanges(nil)
	_ = nilIdx.Reload(ctx)
}
