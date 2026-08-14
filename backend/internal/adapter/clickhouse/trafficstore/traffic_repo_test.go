package trafficstore

import (
	"testing"

	"network_monitor/internal/model"
)

func TestComposeMapAggGeoPathWinsEvenIfEmpty(t *testing.T) {
	got := composeMapAgg("city", nil, true, []model.RawAgg{{SrcIP: "1.1.1.1"}})
	if got.Source != "geo_city" {
		t.Fatalf("source: %s", got.Source)
	}
	if len(got.Raws) != 0 {
		t.Fatalf("raw fallback must not run after geo path, raws=%d", len(got.Raws))
	}
	if got.GeoEdges != nil {
		t.Fatalf("empty geo rows should stay nil, got %#v", got.GeoEdges)
	}
}

func TestComposeMapAggRawWhenGeoUnavailable(t *testing.T) {
	raws := []model.RawAgg{{SrcIP: "10.0.0.1"}}
	got := composeMapAgg("ip", []model.GeoEdgeAgg{{SrcKey: "a"}}, false, raws)
	if got.Source != "ip_live_ip" {
		t.Fatalf("source: %s", got.Source)
	}
	if len(got.Raws) != 1 || got.Raws[0].SrcIP != "10.0.0.1" {
		t.Fatalf("raws: %#v", got.Raws)
	}
	if len(got.GeoEdges) != 0 {
		t.Fatalf("geo edges must be omitted on raw path: %#v", got.GeoEdges)
	}
}
