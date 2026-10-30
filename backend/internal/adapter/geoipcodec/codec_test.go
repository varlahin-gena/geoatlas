package geoipcodec

import (
	"bytes"
	"strings"
	"testing"
)

func TestCodecRoundTrip(t *testing.T) {
	c := New()
	csv := "Network,Country,Region,City,Latitude,Longitude\n1.2.3.0/24,RU,MOW,Moscow,55.75,37.61\n"
	ranges, err := c.ReadCSV(strings.NewReader(csv))
	if err != nil || len(ranges) != 1 {
		t.Fatalf("ReadCSV: %v %v", ranges, err)
	}
	ranges2, snap, err := c.ReadCSVSnapshot(strings.NewReader(csv))
	if err != nil || len(ranges2) != 1 || snap == nil {
		t.Fatalf("snapshot: %v %v %v", ranges2, snap, err)
	}
	var buf bytes.Buffer
	if err := c.WriteCSV(&buf, ranges); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Moscow") {
		t.Fatal(buf.String())
	}
	norm, skipped := c.Normalize(ranges)
	if len(norm) != 1 || skipped != 0 {
		t.Fatalf("norm=%v skipped=%d", norm, skipped)
	}
	if err := c.CheckNonOverlapping(norm); err != nil {
		t.Fatal(err)
	}
	g, err := c.ParseEntry("8.8.8.8/32", "US", "CA", "Mountain View", 37.4, -122.1)
	if err != nil || g.Country != "US" {
		t.Fatalf("%+v %v", g, err)
	}
	start, end, ok := c.ParseNetwork("10.0.0.0/8")
	if !ok || start == 0 || end == 0 {
		t.Fatalf("%d %d %v", start, end, ok)
	}
	if net := c.FormatNetwork(start, end); net == "" {
		t.Fatal("empty format")
	}
}
