package geoip

import (
	"testing"

	"network_monitor/internal/model"
)

func TestCompactBuilderBuildsSnapshotAndSkipsOverlaps(t *testing.T) {
	builder := NewCompactBuilder(4)
	builder.AddRange(model.GeoRange{
		StartIP: 1, EndIP: 10, Country: "RU", Region: "MOW", City: "Moscow", Lat: 55.75, Lon: 37.62,
	})
	builder.AddRange(model.GeoRange{
		StartIP: 5, EndIP: 12, Country: "RU", Region: "MOW", City: "Overlap", Lat: 55.75, Lon: 37.62,
	})
	builder.AddRange(model.GeoRange{
		StartIP: 20, EndIP: 30, Country: "RU", Region: "SPE", City: "Saint Petersburg", Lat: 59.93, Lon: 30.31,
	})

	built := builder.Build()
	if built.Skipped() != 1 {
		t.Fatalf("skipped=%d", built.Skipped())
	}
	if built.RangeCount() != 2 {
		t.Fatalf("count=%d", built.RangeCount())
	}
	if built.ApproxBytes() == 0 {
		t.Fatal("expected approximate bytes")
	}

	idx := New()
	idx.ReplaceBuiltSnapshot(built)
	if got := idx.Lookup(Uint32ToIP(25)); !got.Found || got.City != "Saint Petersburg" {
		t.Fatalf("lookup=%+v", got)
	}
}
