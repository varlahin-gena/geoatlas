package repstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"geoatlas/internal/model"
)

func TestLoadListMetaNilAndOK(t *testing.T) {
	ctx := context.Background()
	if _, err := LoadReputationRanges(ctx, nil); err == nil {
		t.Fatal("nil load")
	}
	if _, err := ListReputationMeta(ctx, nil); err == nil {
		t.Fatal("nil meta")
	}
	if _, err := ReplaceReputationRanges(ctx, nil, nil); err == nil {
		t.Fatal("nil replace")
	}

	ch := &stubConn{}
	ch.pushQuery(repRangeVals("a", 1, 10), repRangeVals("b", 20, 30))
	got, err := LoadReputationRanges(ctx, ch)
	if err != nil || len(got) != 2 {
		t.Fatalf("%v %v", got, err)
	}

	now := time.Now().UTC()
	ch.pushQuery([]any{"a", "malware", uint64(5), "url", now})
	meta, err := ListReputationMeta(ctx, ch)
	if err != nil || len(meta) != 1 || meta[0].Count != 5 {
		t.Fatalf("%+v %v", meta, err)
	}
}

func TestReplaceEmptyAndWithRows(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	// empty ranges: staging fill no-op (no QueryRow)
	n, err := ReplaceReputationRanges(ctx, ch, nil)
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}

	ch2 := &stubConn{}
	ranges := []model.ReputationRange{{
		ListName: "spamhaus", Category: "malware", StartIP: 1, EndIP: 2,
		Source: "url", UpdatedAt: time.Now().UTC(),
	}}
	ch2.pushRow(uint64(1))
	n, err = ReplaceReputationRanges(ctx, ch2, ranges)
	if err != nil || n != 1 || ch2.batchAppend != 1 {
		t.Fatalf("n=%d append=%d err=%v", n, ch2.batchAppend, err)
	}
}

func TestInsertCanceledAndScanError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := &stubConn{}
	if _, err := insertReputationRangesInto(ctx, ch, "t", []model.ReputationRange{{ListName: "a", StartIP: 1, EndIP: 2}}); err == nil {
		t.Fatal("want canceled")
	}
	rows := &stubRows{rows: [][]any{{"bad"}}}
	if _, err := scanReputationRangeRows(rows); err == nil {
		t.Fatal("want scan err")
	}
}

func TestReputationRepositoryPaths(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	r := NewReputationRepository(nil, ch)
	if r.apiConn() != ch {
		t.Fatal("api fallback")
	}
	r2 := NewReputationRepository(ch, ch)
	if r2.apiConn() != ch {
		t.Fatal("api")
	}

	ch.pushQuery(repRangeVals("keep", 1, 2), repRangeVals("drop", 3, 4))
	ch.pushRow(uint64(2)) // keep + new drop after replace list
	n, err := r.ReplaceList(ctx, "drop", []model.ReputationRange{{
		ListName: "drop", Category: "malware", StartIP: 10, EndIP: 20,
		Source: "upload", UpdatedAt: time.Now().UTC(),
	}})
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}

	ch.pushQuery(repRangeVals("keep", 1, 2), repRangeVals("drop", 3, 4))
	ch.pushRow(uint64(1)) // only keep remains
	if err := r.DeleteList(ctx, "drop"); err != nil {
		t.Fatal(err)
	}

	ch.pushQuery(repRangeVals("a", 1, 2))
	if items, err := r.Load(ctx); err != nil || len(items) != 1 {
		t.Fatalf("%v %v", items, err)
	}

	ch.pushRow(uint64(1))
	if n, err := r.ReplaceAll(ctx, []model.ReputationRange{{
		ListName: "a", StartIP: 1, EndIP: 2, Category: "c", Source: "s", UpdatedAt: time.Now().UTC(),
	}}); err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}

	now := time.Now().UTC()
	ch.pushQuery([]any{"a", "c", uint64(2), "s", now})
	if meta, err := r.ListMeta(ctx); err != nil || len(meta) != 1 {
		t.Fatalf("%v %v", meta, err)
	}

	// DeleteList → empty table
	ch.pushQuery(repRangeVals("only", 1, 2))
	if err := r.DeleteList(ctx, "only"); err != nil {
		t.Fatal(err)
	}
}

func TestReloadableReputationIndex(t *testing.T) {
	ctx := context.Background()
	ch := &stubConn{}
	idx := NewReloadableReputationIndex(ch)
	ch.pushQuery(repRangeVals("list", 0x01010101, 0x010101FF))
	if err := idx.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	hits := idx.Lookup("1.1.1.5")
	if len(hits) == 0 {
		t.Fatal("want hit after reload")
	}

	ch.pushQueryErr(errors.New("ch down"))
	if err := idx.Reload(ctx); err == nil {
		t.Fatal("want reload err")
	}

	var nilIdx *ReloadableReputationIndex
	if err := nilIdx.Reload(ctx); err != nil {
		t.Fatal(err)
	}
	if nilIdx.Lookup("1.1.1.1") != nil {
		t.Fatal("nil lookup")
	}
}

func TestLoadQueryError(t *testing.T) {
	ch := &stubConn{}
	ch.pushQueryErr(errors.New("boom"))
	if _, err := LoadReputationRanges(context.Background(), ch); err == nil {
		t.Fatal("want err")
	}
	ch.pushQueryErr(errors.New("boom"))
	if _, err := ListReputationMeta(context.Background(), ch); err == nil {
		t.Fatal("want err")
	}
}
