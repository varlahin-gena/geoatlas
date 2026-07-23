package reputation

import (
	"strings"
	"testing"
	"time"

	"network_monitor/internal/model"
)

func TestParseNetset(t *testing.T) {
	const raw = `
# firehol_level1
# comment
1.2.3.0/24
8.8.8.8
10.0.0.1-10.0.0.5
::1
not-an-ip
`
	ranges, err := ParseNetset(strings.NewReader(raw), "firehol_level1", "firehol", "url", time.Unix(100, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) < 3 {
		t.Fatalf("want >=3 ranges, got %d", len(ranges))
	}
	idx := New()
	idx.ReplaceAll(ranges)
	hits := idx.Lookup("1.2.3.10")
	if len(hits) != 1 || hits[0].List != "firehol_level1" {
		t.Fatalf("lookup: %+v", hits)
	}
	if len(idx.Lookup("9.9.9.9")) != 0 {
		t.Fatal("expected miss")
	}
}

func TestMultiListLookup(t *testing.T) {
	now := time.Now().UTC()
	idx := New()
	idx.ReplaceAll([]model.ReputationRange{
		{ListName: "a", Category: "x", StartIP: IPToUint32("1.1.1.1"), EndIP: IPToUint32("1.1.1.1"), Source: "upload", UpdatedAt: now},
		{ListName: "b", Category: "y", StartIP: IPToUint32("1.1.1.1"), EndIP: IPToUint32("1.1.1.1"), Source: "upload", UpdatedAt: now},
	})
	hits := idx.Lookup("1.1.1.1")
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %+v", hits)
	}
}

func TestReadCSV(t *testing.T) {
	raw := "Network,List,Category\n1.2.3.0/24,custom,malware\n8.8.8.8,custom,dns\n"
	ranges, err := ReadCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 2 {
		t.Fatalf("got %d", len(ranges))
	}
}
