package mapagg

import (
	"strings"
	"testing"

	"geoatlas/internal/model"
)

type lk struct{ g model.GeoLookup }

func (l lk) Lookup(string) model.GeoLookup { return l.g }

func TestIPGroupMetaVariants(t *testing.T) {
	g := lk{g: model.GeoLookup{Found: true, Country: "RU", City: "Moscow", Region: "MOW", Lat: 1, Lon: 2}}
	m := IPGroupMeta(g, "10.1.2.3", "subnet")
	if m.Key == "" || m.Label == "" {
		t.Fatalf("%+v", m)
	}
	m2 := IPGroupMeta(g, "bad", "subnet")
	if m2.Country != "RU" {
		t.Fatalf("%+v", m2)
	}
	m3 := IPGroupMeta(g, "8.8.8.8", "city")
	if !strings.Contains(m3.Key, "Moscow") && m3.Label != "Moscow" {
		t.Fatalf("%+v", m3)
	}
	m4 := IPGroupMeta(g, "8.8.8.8", "country")
	if m4.Key == "" {
		t.Fatalf("%+v", m4)
	}
	_ = IPGroupMeta(g, "8.8.8.8", "continent")
	m6 := IPGroupMeta(nil, "1.1.1.1", "ip")
	if m6.Key != "1.1.1.1" {
		t.Fatalf("%+v", m6)
	}
}
