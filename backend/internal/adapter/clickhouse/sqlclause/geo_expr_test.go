package sqlclause

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGeoKeyExprsContainValidUTF8Unknown(t *testing.T) {
	const unknown = "Неизвестно"
	if !utf8.ValidString(unknown) {
		t.Fatal("fixture is not valid UTF-8")
	}

	exprs := []string{
		CityKeyExpr("src"),
		CityKeyExpr("dst"),
		CityLabelExpr("src"),
		CityLabelExpr("dst"),
		CountryKeyExpr("src"),
		CountryKeyExpr("dst"),
	}
	for _, expr := range exprs {
		if !utf8.ValidString(expr) {
			t.Fatalf("expr is not valid UTF-8: %q", expr)
		}
		if !strings.Contains(expr, unknown) {
			t.Fatalf("expr missing %q: %s", unknown, expr)
		}
		// Старый mojibake от двойной UTF-8 интерпретации.
		if strings.Contains(expr, "РќРµРёР·РІРµСЃС‚РЅРѕ") {
			t.Fatalf("mojibake still present: %s", expr)
		}
	}
	// Без города/страны — ключ/лейбл по IP, не city:unknown.
	ck := CityKeyExpr("src")
	if strings.Contains(ck, "city:unknown") {
		t.Fatalf("CityKeyExpr still uses city:unknown: %s", ck)
	}
	if !strings.Contains(ck, "src_ip") {
		t.Fatalf("CityKeyExpr should fall back to src_ip: %s", ck)
	}
	for _, bad := range []string{"unknown", "Reserved", "reserved"} {
		if !strings.Contains(ck, bad) {
			t.Fatalf("CityKeyExpr should treat %q as bad country: %s", bad, ck)
		}
	}
}

func TestGeoGroupExprsIPAndSubnet(t *testing.T) {
	sk, dk, sl, dl := GeoGroupExprs("ip")
	if sk != "src_ip" || dk != "dst_ip" || sl != "src_ip" || dl != "dst_ip" {
		t.Fatalf("ip exprs: %s %s %s %s", sk, dk, sl, dl)
	}
	sk, dk, _, _ = GeoGroupExprs("subnet")
	if !strings.Contains(sk, "isIPv4String(src_ip)") || !strings.Contains(dk, "isIPv4String(dst_ip)") {
		t.Fatalf("subnet exprs: %s / %s", sk, dk)
	}
	if !strings.Contains(sk, "/24") {
		t.Fatalf("subnet missing /24: %s", sk)
	}
}

func TestGeoEdgesTableAllowlist(t *testing.T) {
	if got := GeoEdgesTable("city"); got != "traffic_edges_city_daily" {
		t.Fatalf("city=%q", got)
	}
	if got := GeoEdgesTable("country"); got != "traffic_edges_country_daily" {
		t.Fatalf("country=%q", got)
	}
	for _, bad := range []string{"", "ip", "subnet", "city; DROP TABLE", "country`x"} {
		if got := GeoEdgesTable(bad); got != "" {
			t.Fatalf("GeoEdgesTable(%q)=%q want empty", bad, got)
		}
		if got := GeoEdgesMV(bad); got != "" {
			t.Fatalf("GeoEdgesMV(%q)=%q want empty", bad, got)
		}
	}
}
