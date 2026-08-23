package mapsearch

import (
	"strings"
	"testing"
)

func TestSanitizeMapCountryClipsAndStripsNUL(t *testing.T) {
	long := strings.Repeat("я", 200)
	if got := SanitizeMapCountry(long); strings.Count(got, "я") != MaxMapCountryRunes {
		t.Fatalf("country clip: %q", got)
	}
	if got := SanitizeMapQuery("ab\x00c"); got != "abc" {
		t.Fatalf("nul: %q", got)
	}
}
