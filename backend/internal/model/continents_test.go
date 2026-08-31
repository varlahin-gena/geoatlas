package model

import "testing"

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
	if expr == "" || expr[:8] != "multiIf(" {
		t.Fatalf("bad expr: %s", expr)
	}
}
