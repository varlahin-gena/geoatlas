package geostore

import (
	"strings"
	"testing"

	"network_monitor/internal/adapter/clickhouse/sqlclause"
)

func TestClipGeoStr(t *testing.T) {
	if got := clipGeoStr("ok"); got != "ok" {
		t.Fatalf("got %q", got)
	}
	long := stringsRepeat("я", geoStrMaxRunes+10)
	got := clipGeoStr(long)
	if len([]rune(got)) != geoStrMaxRunes {
		t.Fatalf("len=%d want %d", len([]rune(got)), geoStrMaxRunes)
	}
}

func stringsRepeat(s string, n int) string {
	b := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		b = append(b, s...)
	}
	return string(b)
}

func TestCountryNeedsSQL(t *testing.T) {
	s := sqlclause.CountryNeedsSQL("src_country")
	for _, want := range []string{"src_country", "unknown", "reserved", "Неизвестно", "lengthUTF8"} {
		if !strings.Contains(s, want) {
			t.Fatalf("CountryNeedsSQL missing %q: %s", want, s)
		}
	}
}
