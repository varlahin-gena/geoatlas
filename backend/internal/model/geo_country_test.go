package model

import "testing"

func TestNeedsCountry(t *testing.T) {
	need := []string{"", "unknown", "Unknown", "UNKNOWN", "Неизвестно", "Reserved", "reserved", "RU", "us", "DE"}
	for _, v := range need {
		if !NeedsCountry(v) {
			t.Fatalf("NeedsCountry(%q) = false, want true", v)
		}
		if UsableCountry(v) {
			t.Fatalf("UsableCountry(%q) = true, want false", v)
		}
	}
	keep := []string{"Russia", "Germany", "United States", "Сингапур"}
	for _, v := range keep {
		if NeedsCountry(v) {
			t.Fatalf("NeedsCountry(%q) = true, want false", v)
		}
		if !UsableCountry(v) {
			t.Fatalf("UsableCountry(%q) = false, want true", v)
		}
	}
}

func TestCountryCenterISO(t *testing.T) {
	for _, code := range []string{"RU", "US", "SG", "DE"} {
		if _, _, ok := CountryCenter(code); !ok {
			t.Fatalf("CountryCenter(%q) missing", code)
		}
	}
}
