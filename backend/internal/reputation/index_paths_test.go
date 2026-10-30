package reputation

import (
	"strings"
	"testing"
	"time"

	"geoatlas/internal/model"
)

func TestIndexReplaceDeleteMetaSnapshot(t *testing.T) {
	idx := New()
	if idx.RangeCount() != 0 || idx.Snapshot() != nil {
		t.Fatal("empty")
	}
	now := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	idx.ReplaceAll([]model.ReputationRange{
		{ListName: "a", Category: "malware", StartIP: 0x01010100, EndIP: 0x010101ff, Source: "upload", UpdatedAt: now},
		{ListName: "b", Category: "botnet", StartIP: 0x02020200, EndIP: 0x020202ff, Source: "url", UpdatedAt: now},
	})
	if idx.RangeCount() != 2 {
		t.Fatal(idx.RangeCount())
	}
	idx.ReplaceList("a", []model.ReputationRange{
		{Category: "malware", StartIP: 0x01010100, EndIP: 0x0101017f, Source: "upload", UpdatedAt: now},
		{Category: "malware", StartIP: 0x01010100, EndIP: 0x010101ff, Source: "upload", UpdatedAt: now}, // wider + narrower same key
	})
	hits := idx.Lookup("1.1.1.1")
	if len(hits) == 0 {
		t.Fatal("lookup")
	}
	meta := idx.ListMeta()
	if len(meta) < 2 {
		t.Fatalf("%+v", meta)
	}
	snap := idx.Snapshot()
	if len(snap) == 0 {
		t.Fatal("snap")
	}
	idx.DeleteList("b")
	if idx.RangeCount() != 1 {
		t.Fatal(idx.RangeCount())
	}
	var nilIdx *Index
	if nilIdx.Lookup("1.1.1.1") != nil {
		t.Fatal("nil lookup")
	}
}

func TestFormatNetworkAndNormalizeEdges(t *testing.T) {
	if FormatNetwork(0x01020304, 0x01020304) == "" {
		t.Fatal("format single")
	}
	if FormatNetworkPreferCIDR(0x0a000000, 0x0affffff) != "10.0.0.0/8" {
		t.Fatal(FormatNetworkPreferCIDR(0x0a000000, 0x0affffff))
	}
	// non-aligned span → a-b form
	got := FormatNetworkPreferCIDR(0x01020304, 0x01020306)
	if !strings.Contains(got, "-") && got == "" {
		t.Fatal(got)
	}
	_, ok := exactPrefixLen(5, 3)
	if ok {
		t.Fatal("start>end")
	}
	if itoa(0) != "0" || itoa(32) != "32" {
		t.Fatal(itoa(0), itoa(32))
	}

	out := NormalizeRanges([]model.ReputationRange{
		{}, // empty list skipped
		{ListName: "x", Category: "c", StartIP: 10, EndIP: 5}, // bad range
		{ListName: "x", Category: "c", StartIP: 1, EndIP: 5, Source: "", UpdatedAt: time.Unix(1, 0)},
		{ListName: "x", Category: "c", StartIP: 6, EndIP: 10, Source: "s", UpdatedAt: time.Unix(2, 0)}, // adjacent merge
	})
	if len(out) != 1 || out[0].EndIP != 10 || out[0].Source != "s" {
		t.Fatalf("%+v", out)
	}
	if NormalizeRanges(nil) != nil {
		t.Fatal("nil")
	}
}

func TestParseFeedBodyUnsupportedAndCSVIPPaths(t *testing.T) {
	_, err := ParseFeedBody("weird", strings.NewReader("x"), "l", "c", "s", time.Now())
	if err == nil {
		t.Fatal("unsupported")
	}
	_, err = ParseFeedBody("csv_ip", strings.NewReader("cidr\n1.2.3.0/24\n"), "list", "cat", "src", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseFeedBody("csv_ip", strings.NewReader("1.2.3.0/24\n4.5.6.0/24\n"), "list", "cat", "src", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseFeedBody("csv_ip", strings.NewReader("name,value\nfoo,bar\n"), "list", "cat", "src", time.Now())
	if err == nil {
		t.Fatal("no ips")
	}
	_, err = ParseCSVIP(strings.NewReader("ip\n"), "", "c", "s", time.Now())
	if err == nil {
		t.Fatal("empty list")
	}
	col, data := detectIPColumn([]string{"ip_address"})
	if col != 0 || data {
		t.Fatalf("%d %v", col, data)
	}
	col, data = detectIPColumn([]string{"x", "8.8.8.8"})
	if !data || col != 1 {
		t.Fatalf("%d %v", col, data)
	}
}
