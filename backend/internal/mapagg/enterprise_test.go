package mapagg

import (
	"testing"

	"geoatlas/internal/model"
)

func TestEnterpriseIndexLookup(t *testing.T) {
	idx := NewEnterpriseIndex([]model.EnterpriseNet{
		{StartIP: 0x0a000000, EndIP: 0x0a00ffff, Network: "10.0.0.0/16", City: "Moscow", Country: "Russia"},
		{StartIP: 0x0a000000, EndIP: 0x0affffff, Network: "10.0.0.0/8", City: "HQ", Country: "Russia"},
	})
	hit, ok := idx.Lookup("10.0.1.5")
	if !ok {
		t.Fatal("expected hit")
	}
	// Более узкий диапазон с тем же Start встречается первым при обратном обходе —
	// любой hit допустим; главное — Contains.
	if hit.Country != "Russia" {
		t.Fatalf("country=%q", hit.Country)
	}
	if !idx.Contains("10.0.1.5") {
		t.Fatal("Contains")
	}
	if idx.Contains("8.8.8.8") {
		t.Fatal("public IP must miss")
	}
	if !NewEnterpriseIndex(nil).Empty() {
		t.Fatal("nil nets empty")
	}
	if _, ok := (*EnterpriseIndex)(nil).Lookup("10.0.0.1"); ok {
		t.Fatal("nil index miss")
	}
}

func TestIPGroupMetaHybrid(t *testing.T) {
	geo := stubGeo{
		"10.0.0.1": {Found: true, Lat: 55.75, Lon: 37.62, City: "Moscow", Country: "Russia"},
		"8.8.8.8":  {Found: true, Lat: 37.4, Lon: -122.1, City: "Mountain View", Country: "United States"},
		"10.0.0.2": {Found: true, Lat: 55.0, Lon: 37.0, Country: "Russia"},
	}

	ent := IPGroupMetaHybrid(geo, "10.0.0.1", true, LogGeoHint{})
	if ent.Key != "Moscow, Russia" || ent.Label != "Moscow" {
		t.Fatalf("enterprise city key=%q label=%q", ent.Key, ent.Label)
	}

	pub := IPGroupMetaHybrid(geo, "8.8.8.8", false, LogGeoHint{})
	if pub.Key != "United States" {
		t.Fatalf("public country key=%q", pub.Key)
	}

	// Enterprise без города → страна.
	noCity := IPGroupMetaHybrid(geo, "10.0.0.2", true, LogGeoHint{})
	if noCity.Key != "Russia" {
		t.Fatalf("no-city fallback key=%q want Russia", noCity.Key)
	}

	// Enterprise city из hint, если geo без города.
	fromHint := IPGroupMetaHybrid(geo, "10.0.0.2", true, LogGeoHint{City: "Kazan", Country: "Russia"})
	if fromHint.Key != "Kazan, Russia" || fromHint.Label != "Kazan" {
		t.Fatalf("hint city key=%q label=%q", fromHint.Key, fromHint.Label)
	}
}
