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
		cityKeyExpr("src", ""),
		cityKeyExpr("dst", ""),
		cityLabelExpr("src", ""),
		cityLabelExpr("dst", ""),
		countryKeyExpr("src", ""),
		countryKeyExpr("dst", ""),
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
	ck := cityKeyExpr("src", "")
	if strings.Contains(ck, "city:unknown") {
		t.Fatalf("cityKeyExpr still uses city:unknown: %s", ck)
	}
	if !strings.Contains(ck, "src_ip") {
		t.Fatalf("cityKeyExpr should fall back to src_ip: %s", ck)
	}
	if !strings.Contains(ck, "toString(") {
		t.Fatalf("cityKeyExpr IP fallback must toString IPv4: %s", ck)
	}
	for _, bad := range []string{"unknown", "Reserved", "reserved"} {
		if !strings.Contains(ck, bad) {
			t.Fatalf("cityKeyExpr should treat %q as bad country: %s", bad, ck)
		}
	}
}

func TestGeoGroupExprsIPAndSubnet(t *testing.T) {
	sk, dk, sl, dl := GeoGroupExprs("ip")
	if sk != "toString(src_ip)" || dk != "toString(dst_ip)" || sl != "toString(src_ip)" || dl != "toString(dst_ip)" {
		t.Fatalf("ip exprs: %s %s %s %s", sk, dk, sl, dl)
	}
	sk, dk, _, _ = GeoGroupExprs("subnet")
	if !strings.Contains(sk, "toUInt32(src_ip)") || !strings.Contains(dk, "toUInt32(dst_ip)") {
		t.Fatalf("subnet exprs: %s / %s", sk, dk)
	}
	if strings.Contains(sk, "isIPv4String") || strings.Contains(sk, "IPv4StringToNum") {
		t.Fatalf("subnet must not use String IP helpers: %s", sk)
	}
	if !strings.Contains(sk, "/24") {
		t.Fatalf("subnet missing /24: %s", sk)
	}
}

func TestGeoGroupExprsPrefixedQualifiesCityCols(t *testing.T) {
	sk, dk, sl, dl := GeoGroupExprsPrefixed("traffic_logs", "city")
	for i, expr := range []string{sk, dk, sl, dl} {
		if !strings.Contains(expr, "traffic_logs.") {
			t.Fatalf("expr[%d] not prefixed: %s", i, expr)
		}
		// Bare column names would be shadowed by anyState(...) AS src_city in MV SELECT.
		if strings.Contains(expr, "trimBoth(src_city)") || strings.Contains(expr, "trimBoth(dst_city)") ||
			strings.Contains(expr, "trimBoth(src_country)") || strings.Contains(expr, "trimBoth(dst_country)") {
			t.Fatalf("bare trimBoth column would alias-shadow in MV: %s", expr)
		}
	}
	if !strings.Contains(sk, "traffic_logs.src_city") || !strings.Contains(dk, "traffic_logs.dst_city") {
		t.Fatalf("city keys missing qualified city cols: %s / %s", sk, dk)
	}
	sk, _, _, _ = GeoGroupExprsPrefixed("traffic_logs", "country")
	if !strings.Contains(sk, "traffic_logs.src_country") {
		t.Fatalf("country key not prefixed: %s", sk)
	}
	if strings.Contains(sk, "trimBoth(src_country)") {
		t.Fatalf("bare src_country in country key: %s", sk)
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
