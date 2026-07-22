package geoip

import (
	"strings"
	"testing"

	"network_monitor/internal/model"
)

func TestFindContainingRange(t *testing.T) {
	ranges := []model.GeoRange{
		{StartIP: IPToUint32("10.0.0.0"), EndIP: IPToUint32("10.0.0.255"), Country: "RU", City: "A"},
		{StartIP: IPToUint32("192.168.1.0"), EndIP: IPToUint32("192.168.1.255"), Country: "RU", City: "B"},
	}
	clean, _ := NormalizeRanges(ranges)
	g, ok := FindContainingRange(clean, "10.0.0.42")
	if !ok || g.City != "A" {
		t.Fatalf("hit: ok=%v %+v", ok, g)
	}
	if _, ok := FindContainingRange(clean, "8.8.8.8"); ok {
		t.Fatal("expected miss")
	}
	if _, ok := FindContainingRange(clean, "not-ip"); ok {
		t.Fatal("invalid ip")
	}
}

func TestLookupIPv4HitMiss(t *testing.T) {
	idx := New()
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: IPToUint32("10.0.0.0"), EndIP: IPToUint32("10.0.0.255"), Country: "RU", City: "MSK", Lat: 55, Lon: 37},
		{StartIP: IPToUint32("192.168.1.0"), EndIP: IPToUint32("192.168.1.255"), Country: "RU", City: "SPB", Lat: 59, Lon: 30},
	})

	got := idx.Lookup("10.0.0.42")
	if !got.Found || got.City != "MSK" {
		t.Fatalf("lookup hit: %+v", got)
	}
	miss := idx.Lookup("8.8.8.8")
	if miss.Found || miss.Country != "Неизвестно" {
		t.Fatalf("lookup miss: %+v", miss)
	}
}

func TestLookupInclusiveBoundaries(t *testing.T) {
	idx := New()
	start := IPToUint32("10.0.0.0")
	end := IPToUint32("10.0.0.255")
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: start, EndIP: end, Country: "RU", City: "A", Lat: 1, Lon: 2},
	})

	for _, ip := range []string{"10.0.0.0", "10.0.0.255"} {
		got := idx.Lookup(ip)
		if !got.Found || got.City != "A" {
			t.Fatalf("%s: %+v", ip, got)
		}
	}
	before := idx.Lookup("9.255.255.255")
	if before.Found {
		t.Fatalf("before range: %+v", before)
	}
	after := idx.Lookup("10.0.1.0")
	if after.Found {
		t.Fatalf("after range: %+v", after)
	}
}

func TestLookupSparseGapBetweenRanges(t *testing.T) {
	idx := New()
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: IPToUint32("10.0.0.0"), EndIP: IPToUint32("10.0.0.10"), Country: "A", City: "low"},
		{StartIP: IPToUint32("10.0.0.20"), EndIP: IPToUint32("10.0.0.30"), Country: "B", City: "high"},
	})
	gap := idx.Lookup("10.0.0.15")
	if gap.Found {
		t.Fatalf("gap should miss: %+v", gap)
	}
	low := idx.Lookup("10.0.0.5")
	if !low.Found || low.City != "low" {
		t.Fatalf("low: %+v", low)
	}
	high := idx.Lookup("10.0.0.25")
	if !high.Found || high.City != "high" {
		t.Fatalf("high: %+v", high)
	}
}

func TestLookupAfterOverlapNormalizeKeepsFirst(t *testing.T) {
	idx := New()
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: 100, EndIP: 200, Country: "A", City: "first"},
		{StartIP: 150, EndIP: 250, Country: "B", City: "overlap"},
	})
	got := idx.Lookup(Uint32ToIP(175))
	if !got.Found || got.City != "first" {
		t.Fatalf("want first range after normalize, got %+v", got)
	}
}

func TestLookupRejectsIPv6(t *testing.T) {
	idx := New()
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: 1, EndIP: 2, Country: "X", City: "Y"},
	})
	got := idx.Lookup("2001:db8::1")
	if got.Found {
		t.Fatalf("IPv6 should not match: %+v", got)
	}
}

func TestNormalizeRangesDropsOverlaps(t *testing.T) {
	clean, skipped := NormalizeRanges([]model.GeoRange{
		{StartIP: 100, EndIP: 200, Country: "A"},
		{StartIP: 150, EndIP: 250, Country: "B"}, // overlaps A
		{StartIP: 300, EndIP: 400, Country: "C"},
	})
	if skipped != 1 || len(clean) != 2 {
		t.Fatalf("clean=%d skipped=%d", len(clean), skipped)
	}
	if clean[0].Country != "A" || clean[1].Country != "C" {
		t.Fatalf("unexpected keep order: %+v", clean)
	}
}

func TestCheckNonOverlapping(t *testing.T) {
	ok := []model.GeoRange{
		{StartIP: 1, EndIP: 10},
		{StartIP: 11, EndIP: 20},
	}
	if err := CheckNonOverlapping(ok); err != nil {
		t.Fatal(err)
	}
	bad := []model.GeoRange{
		{StartIP: 1, EndIP: 10},
		{StartIP: 10, EndIP: 20}, // touch at boundary = overlap (inclusive)
	}
	if err := CheckNonOverlapping(bad); err == nil {
		t.Fatal("expected overlap error")
	}
}

func TestReadCSVRejectsOverlaps(t *testing.T) {
	csv := `Network,Country,Region,City,Latitude,Longitude
10.0.0.0-10.0.0.255,RU,X,A,55,37
10.0.0.128-10.0.1.0,RU,X,B,55,37
`
	_, err := ReadCSV(strings.NewReader(csv))
	if err == nil {
		t.Fatal("expected overlap error from ReadCSV")
	}
}

func TestReadCSVAcceptsAdjacent(t *testing.T) {
	csv := `Network,Country,Region,City,Latitude,Longitude
10.0.0.0-10.0.0.255,RU,X,A,55,37
10.0.1.0-10.0.1.255,RU,X,B,55,37
`
	ranges, err := ReadCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d ranges", len(ranges))
	}
}

func TestParseRangeEntryAndWriteCSVRoundTrip(t *testing.T) {
	g, err := ParseRangeEntry("10.0.0.5", "Россия", "Org", "Москва", 55.75, 37.62)
	if err != nil {
		t.Fatal(err)
	}
	if g.StartIP != g.EndIP || FormatNetwork(g.StartIP, g.EndIP) != "10.0.0.5" {
		t.Fatalf("single IP: %+v", g)
	}

	var buf strings.Builder
	if err := WriteCSV(&buf, []model.GeoRange{g}); err != nil {
		t.Fatal(err)
	}
	back, err := ReadCSV(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 1 || back[0].City != "Москва" || back[0].Lat != 55.75 {
		t.Fatalf("round-trip: %+v", back)
	}
}

func TestParseRangeEntryValidation(t *testing.T) {
	if _, err := ParseRangeEntry("not-an-ip", "RU", "R", "C", 0, 0); err == nil {
		t.Fatal("want invalid network")
	}
	if _, err := ParseRangeEntry("1.2.3.4", "", "R", "C", 0, 0); err == nil {
		t.Fatal("want required fields")
	}
	if _, err := ParseRangeEntry("1.2.3.4", "RU", "R", "C", 91, 0); err == nil {
		t.Fatal("want lat bounds")
	}
}

func TestCollectRangesAndLookupRange(t *testing.T) {
	idx := New()
	idx.ReplaceRanges([]model.GeoRange{
		{StartIP: 1, EndIP: 10, Country: "A", City: "One"},
		{StartIP: 20, EndIP: 30, Country: "B", City: "Two"},
		{StartIP: 40, EndIP: 50, Country: "C", City: "Three"},
	})
	items, total, filtered, trunc := idx.CollectRanges(2, "")
	if total != 3 || filtered != 3 || !trunc || len(items) != 2 {
		t.Fatalf("page=%d total=%d filtered=%d trunc=%v", len(items), total, filtered, trunc)
	}
	items, total, filtered, trunc = idx.CollectRanges(10, "two")
	if total != 3 || filtered != 1 || trunc || len(items) != 1 || items[0].City != "Two" {
		t.Fatalf("search items=%#v total=%d filtered=%d trunc=%v", items, total, filtered, trunc)
	}
	g, ok := idx.LookupRange(Uint32ToIP(25))
	if !ok || g.Country != "B" {
		t.Fatalf("LookupRange: ok=%v g=%#v", ok, g)
	}
	if _, ok := idx.LookupRange("8.8.8.8"); ok {
		t.Fatal("expected miss")
	}
}
