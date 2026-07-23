package clickhouse

import (
	"context"
	"strings"
	"testing"
)

func TestQuoteCHString(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "''"},
		{"Moscow", "'Moscow'"},
		{"O'Brien", "'O\\'Brien'"},
		{`a\b`, `'a\\b'`},
		{"a\\'b", "'a\\\\\\'b'"}, // a \ ' b → a \\ \' b
		{"x\x00y", "'xy'"},
	}
	for _, tc := range cases {
		if got := quoteCHString(tc.in); got != tc.want {
			t.Fatalf("quoteCHString(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

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

func TestMutateGeoSideRejectsBadSide(t *testing.T) {
	err := mutateGeoSide(context.TODO(), nil, "evil", []geoEnrichRow{{ip: "1.1.1.1", lat: 1}})
	if err == nil {
		t.Fatal("expected error for invalid side")
	}
}

func TestCountryNeedsSQL(t *testing.T) {
	s := countryNeedsSQL("src_country")
	for _, want := range []string{"src_country", "unknown", "reserved", "Неизвестно", "lengthUTF8"} {
		if !strings.Contains(s, want) {
			t.Fatalf("countryNeedsSQL missing %q: %s", want, s)
		}
	}
}
