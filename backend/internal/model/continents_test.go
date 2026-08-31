package model

import (
	"strings"
	"testing"
)

func TestContinentOfKnownCountries(t *testing.T) {
	cases := map[string]string{
		"Russia":        ContinentEurope,
		"Россия":        ContinentEurope,
		"Germany":       ContinentEurope,
		"United States": ContinentNorthAmerica,
		"США":           ContinentNorthAmerica,
		"China":         ContinentAsia,
		"Brazil":        ContinentSouthAmerica,
		"Australia":     ContinentOceania,
		"Egypt":         ContinentAfrica,
	}
	for country, want := range cases {
		if got := ContinentOf(country); got != want {
			t.Fatalf("ContinentOf(%q) = %q, want %q", country, got, want)
		}
	}
}

func TestContinentOfUnknown(t *testing.T) {
	if got := ContinentOf(""); got != ContinentUnknown {
		t.Fatalf("empty = %q", got)
	}
	if got := ContinentOf("Неизвестно"); got != ContinentUnknown {
		t.Fatalf("Неизвестно = %q", got)
	}
}

func TestContinentCenter(t *testing.T) {
	lat, lon, ok := ContinentCenter(ContinentEurope)
	if !ok || lat == 0 || lon == 0 {
		t.Fatalf("Europe center missing: %v %v %v", lat, lon, ok)
	}
}

func TestContinentSQLExpr(t *testing.T) {
	expr := ContinentSQLExpr("src_country")
	if expr == "" || !strings.HasPrefix(expr, "transform(src_country, [") {
		t.Fatalf("bad expr prefix: %.80q", expr)
	}
	if strings.Count(expr, "src_country") != 1 {
		t.Fatalf("country expr should appear once, got %d", strings.Count(expr, "src_country"))
	}
	if len(expr) > 32000 {
		t.Fatalf("expr too long: %d bytes", len(expr))
	}
	if !strings.Contains(expr, "'Russia'") || !strings.Contains(expr, "'Европа'") {
		t.Fatalf("missing sample mapping in: %.120q", expr)
	}
}
