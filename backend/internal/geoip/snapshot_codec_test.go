package geoip

import (
	"bytes"
	"testing"

	"geoatlas/internal/model"
)

func TestSnapshotCodecRoundTrip(t *testing.T) {
	builder := NewCompactBuilder(4)
	builder.AddRange(model.GeoRange{
		StartIP: 1, EndIP: 10, Country: "RU", Region: "MOW", City: "Moscow", Lat: 55.75, Lon: 37.62,
	})
	builder.AddRange(model.GeoRange{
		StartIP: 20, EndIP: 30, Country: "US", Region: "CA", City: "San Francisco", Lat: 37.77, Lon: -122.42,
	})
	built := builder.Build()
	stamp := StampFromRanges([]model.GeoRange{
		{StartIP: 1, EndIP: 10},
		{StartIP: 20, EndIP: 30},
	})

	raw, err := EncodeSnapshot(built, stamp)
	if err != nil {
		t.Fatal(err)
	}
	got, gotStamp, err := DecodeSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !gotStamp.Equal(stamp) {
		t.Fatalf("stamp %+v vs %+v", gotStamp, stamp)
	}
	if got.RangeCount() != 2 {
		t.Fatalf("count=%d", got.RangeCount())
	}
	idx := New()
	idx.ReplaceBuiltSnapshot(got)
	lu := idx.Lookup("0.0.0.5")
	if !lu.Found || lu.City != "Moscow" {
		t.Fatalf("lookup: %+v", lu)
	}
	lu = idx.Lookup("0.0.0.25")
	if !lu.Found || lu.City != "San Francisco" {
		t.Fatalf("lookup sf: %+v", lu)
	}
}

func TestDecodeSnapshotRejectsCorruption(t *testing.T) {
	built := NewCompactBuilder(1).Build()
	raw, err := EncodeSnapshot(built, SourceStamp{})
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	if _, _, err := DecodeSnapshot(raw); err == nil {
		t.Fatal("expected checksum error")
	}
	if _, _, err := DecodeSnapshot([]byte("nope")); err == nil {
		t.Fatal("expected truncated")
	}
}

func TestEncodeEmptySnapshotStable(t *testing.T) {
	a, err := EncodeSnapshot(nil, SourceStamp{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := EncodeSnapshot(&BuiltSnapshot{}, SourceStamp{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("nil and empty snapshot must encode the same")
	}
}
