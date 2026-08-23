package mapagg

import (
	"strings"
	"testing"

	"geoatlas/internal/model"
)

type stubGeo map[string]model.GeoLookup

func (s stubGeo) Lookup(ipStr string) model.GeoLookup {
	if lk, ok := s[ipStr]; ok {
		return lk
	}
	return model.GeoLookup{Country: "Неизвестно"}
}

func TestIPGroupMetaHintedUsesStoredCoordsWhenLookupMisses(t *testing.T) {
	m := IPGroupMetaHinted(stubGeo{}, "10.0.0.1", "ip", LogGeoHint{
		Lat: 55.75, Lon: 37.62, Country: "Russia", City: "Moscow",
	})
	if !m.Valid {
		t.Fatal("expected Valid")
	}
	if m.Lat != 55.75 || m.Lon != 37.62 {
		t.Fatalf("coords = (%v,%v)", m.Lat, m.Lon)
	}
	if m.Key != "10.0.0.1" {
		t.Fatalf("key = %q", m.Key)
	}
}

func TestIPGroupMetaHintedSubnetUsesCountryCenter(t *testing.T) {
	m := IPGroupMetaHinted(stubGeo{}, "192.168.1.10", "subnet", LogGeoHint{
		Country: "Germany",
	})
	if !m.Valid {
		t.Fatal("expected Valid via CountryCenter")
	}
	if m.Key != "192.168.1.0/24" {
		t.Fatalf("key = %q", m.Key)
	}
	cLat, cLon, ok := model.CountryCenter("Germany")
	if !ok {
		t.Fatal("Germany center missing")
	}
	if m.Lat != cLat || m.Lon != cLon {
		t.Fatalf("coords = (%v,%v), want (%v,%v)", m.Lat, m.Lon, cLat, cLon)
	}
}

func TestIPGroupMetaHintedPrefersLiveLookup(t *testing.T) {
	geo := stubGeo{
		"1.1.1.1": {Lat: 1, Lon: 2, City: "Sydney", Country: "Australia", Found: true},
	}
	m := IPGroupMetaHinted(geo, "1.1.1.1", "ip", LogGeoHint{
		Lat: 55, Lon: 37, Country: "Russia",
	})
	if m.Lat != 1 || m.Lon != 2 {
		t.Fatalf("should keep live coords, got (%v,%v)", m.Lat, m.Lon)
	}
}

func TestIPGroupMetaFillsCountryWhenLiveCoordsExist(t *testing.T) {
	// Live GeoIP дал coords, но без country — страна из лога должна подставиться.
	geo := stubGeo{
		"8.8.8.8": {Lat: 37.4, Lon: -122.1, City: "Mountain View", Found: true},
	}
	m := IPGroupMetaHinted(geo, "8.8.8.8", "city", LogGeoHint{
		Country: "United States", City: "Mountain View",
	})
	if !m.Valid {
		t.Fatal("expected Valid")
	}
	if m.Country != "United States" {
		t.Fatalf("country = %q, want United States", m.Country)
	}
	if !strings.Contains(m.Key, "Mountain View") {
		t.Fatalf("key = %q, want Mountain View*", m.Key)
	}
}

func TestPreferCountryReplacesReserved(t *testing.T) {
	if got := preferCountry("Reserved", "Russia"); got != "Russia" {
		t.Fatalf("got %q", got)
	}
	if got := preferCountry("RU", "Russia"); got != "Russia" {
		t.Fatalf("got %q", got)
	}
	if got := preferCountry("Germany", "Неизвестно"); got != "Germany" {
		t.Fatalf("got %q", got)
	}
}

func TestIPGroupMetaCityWithoutPlaceUsesIPKey(t *testing.T) {
	// Found + coords, но без city/country — раньше оба IP шли в city:unknown
	// и дуга становилась невидимым self-loop.
	geo := stubGeo{
		"203.0.113.10": {Lat: 33.3, Lon: 44.4, Found: true},
		"198.51.100.7": {Lat: 1.2, Lon: 103.8, Found: true},
	}
	a := IPGroupMetaHinted(geo, "203.0.113.10", "city", LogGeoHint{})
	b := IPGroupMetaHinted(geo, "198.51.100.7", "city", LogGeoHint{})
	if !a.Valid || !b.Valid {
		t.Fatal("expected Valid")
	}
	if a.Key != "203.0.113.10" || b.Key != "198.51.100.7" {
		t.Fatalf("keys = %q, %q", a.Key, b.Key)
	}
	if a.Key == b.Key {
		t.Fatal("IPs collapsed into one map key")
	}
}

func TestIPGroupMetaCityHintPromotesUnknownKey(t *testing.T) {
	m := IPGroupMetaHinted(stubGeo{}, "155.212.245.143", "city", LogGeoHint{
		Lat: 1.3, Lon: 103.8, City: "Singapore", Country: "Singapore",
	})
	if !m.Valid {
		t.Fatal("expected Valid")
	}
	if m.Key != "Singapore, Singapore" && m.Key != "Singapore" {
		// city + country → "Singapore, Singapore"
		if !strings.Contains(m.Key, "Singapore") {
			t.Fatalf("key = %q, want Singapore*", m.Key)
		}
	}
	if m.Key == "city:unknown" {
		t.Fatal("key still city:unknown")
	}
}

func TestBuildMapFromGeoEdges(t *testing.T) {
	rows := []model.GeoEdgeAgg{{
		SrcKey: "Berlin, Germany", DstKey: "United States",
		SrcLabel: "Berlin", DstLabel: "United States",
		SrcCity: "Berlin", SrcCountry: "Germany",
		DstCountry: "United States",
		SrcLat: 52.5, SrcLon: 13.4, DstLat: 39.0, DstLon: -77.0,
		Count: 10, AllowedCnt: 10,
		LastAction: "allow",
	}}
	lines, points, skipped := BuildMapFromGeoEdges(rows)
	if skipped != 0 {
		t.Fatalf("skipped = %d", skipped)
	}
	if len(lines) != 1 {
		t.Fatalf("lines = %d", len(lines))
	}
	if len(points) != 2 {
		t.Fatalf("points = %d", len(points))
	}
	if lines[0].Src != "Berlin, Germany" {
		t.Fatalf("src = %q", lines[0].Src)
	}
}

func TestBuildMapFromGeoEdgesCityCountryUsesCountryCenter(t *testing.T) {
	// Усреднённые IP-координаты «безымянных» канадских адресов — восток,
	// но ключ city:country:Canada должен сидеть в центре страны.
	rows := []model.GeoEdgeAgg{{
		SrcKey: "city:country:Canada", DstKey: "Moscow, Russia",
		SrcLabel: "Canada", DstLabel: "Moscow",
		SrcCountry: "Canada", DstCity: "Moscow", DstCountry: "Russia",
		SrcLat: 43.65, SrcLon: -79.38, // Toronto
		DstLat: 55.75, DstLon: 37.62,
		Count: 8, AllowedCnt: 8,
		LastAction: "allow",
	}}
	lines, points, skipped := BuildMapFromGeoEdges(rows)
	if skipped != 0 {
		t.Fatalf("skipped = %d", skipped)
	}
	ca, ok := points["city:country:Canada"]
	if !ok {
		t.Fatal("missing Canada node")
	}
	if ca.Lat < 55 || ca.Lat > 57 || ca.Lon > -105 || ca.Lon < -108 {
		t.Fatalf("Canada node at (%v,%v), want CountryCenter ~ (56.13,-106.35)", ca.Lat, ca.Lon)
	}
	if lines[0].SrcLat < 55 || lines[0].SrcLon > -105 {
		t.Fatalf("line src at (%v,%v), want Canada CountryCenter", lines[0].SrcLat, lines[0].SrcLon)
	}
}

func TestBuildMapFromGeoEdgesSkipsUnknown(t *testing.T) {
	rows := []model.GeoEdgeAgg{{
		SrcKey: "city:unknown", DstKey: "United States",
		SrcLabel: "Неизвестно", DstLabel: "United States",
		Count: 5, AllowedCnt: 5,
	}}
	lines, _, skipped := BuildMapFromGeoEdges(rows)
	if len(lines) != 0 {
		t.Fatalf("lines = %d, want 0", len(lines))
	}
	if skipped == 0 {
		t.Fatal("expected skipped")
	}
}
