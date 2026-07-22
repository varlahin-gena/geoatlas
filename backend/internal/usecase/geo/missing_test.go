package geo

import (
	"testing"

	"network_monitor/internal/model"
	"network_monitor/internal/mapagg"
)

type stubGeoMissing map[string]model.GeoLookup

func (s stubGeoMissing) Lookup(ipStr string) model.GeoLookup {
	if lk, ok := s[ipStr]; ok {
		return lk
	}
	return model.GeoLookup{Country: "Неизвестно"}
}

func TestClassifyIPKind(t *testing.T) {
	cases := map[string]string{
		"10.0.0.1":    "private",
		"192.168.1.1": "private",
		"127.0.0.1":   "loopback",
		"8.8.8.8":     "public_unknown",
		"not-an-ip":   "invalid",
		"224.0.0.1":   "link_local",
		"239.1.1.1":   "multicast",
		"169.254.1.1": "link_local",
	}
	for ip, want := range cases {
		if got := classifyIPKind(ip); got != want {
			t.Errorf("classifyIPKind(%q)=%q, want %q", ip, got, want)
		}
	}
}

func TestFilterGeoMissingRows(t *testing.T) {
	geo := stubGeoMissing{
		"1.1.1.1": {Found: true, Lat: 1, Lon: 2, Country: "AU", City: "Sydney"},
	}
	rows := []model.GeoMissingIPRow{
		{IP: "10.0.0.5", Count: 10, AsSrc: 10},
		{IP: "8.8.8.8", Count: 3, AsDst: 3},
		{IP: "1.1.1.1", Count: 99},
	}
	items := filterGeoMissingRows(rows, geo)
	if len(items) != 2 {
		t.Fatalf("items=%d want 2", len(items))
	}
	if items[0].IP != "10.0.0.5" || items[0].Kind != "private" {
		t.Fatalf("top=%#v", items[0])
	}
	if items[1].IP != "8.8.8.8" || items[1].Kind != "public_unknown" {
		t.Fatalf("second=%#v", items[1])
	}
}

func TestFilterGeoMissingUsesLogCoords(t *testing.T) {
	geo := stubGeoMissing{}
	rows := []model.GeoMissingIPRow{
		{
			IP: "203.0.113.10", Count: 5,
			LogLat: 55.75, LogLon: 37.61, LogCountry: "Russia", LogCity: "Moscow",
		},
		{IP: "203.0.113.20", Count: 2},
	}
	items := filterGeoMissingRows(rows, geo)
	if len(items) != 1 || items[0].IP != "203.0.113.20" {
		t.Fatalf("items=%#v", items)
	}
}

func TestIPHasMapCoords(t *testing.T) {
	geo := stubGeoMissing{
		"9.9.9.9": {Found: true, Lat: 10, Lon: 20, Country: "US"},
	}
	if !ipHasMapCoords(geo, "9.9.9.9", mapagg.LogGeoHint{}) {
		t.Fatal("expected live hit")
	}
	if ipHasMapCoords(geo, "10.1.1.1", mapagg.LogGeoHint{}) {
		t.Fatal("private without coords should fail")
	}
}

func TestIPHasMapCoordsIgnoresCountryOnly(t *testing.T) {
	geo := stubGeoMissing{}
	if ipHasMapCoords(geo, "195.178.110.137", mapagg.LogGeoHint{Country: "Russia"}) {
		t.Fatal("country-only hint must not count as map coords")
	}
	rows := []model.GeoMissingIPRow{
		{IP: "195.178.110.137", Count: 5, LogCountry: "Russia"},
	}
	items := filterGeoMissingRows(rows, geo)
	if len(items) != 1 || items[0].IP != "195.178.110.137" {
		t.Fatalf("items=%#v", items)
	}
}
