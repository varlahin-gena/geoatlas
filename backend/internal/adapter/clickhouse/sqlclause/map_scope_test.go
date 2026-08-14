package sqlclause

import (
	"strings"
	"testing"
)

func TestSanitizeMapScopeClipsAndStripsNUL(t *testing.T) {
	long := strings.Repeat("я", 200)
	if got := SanitizeMapCountry(long); got == "" || strings.Count(got, "я") != maxMapCountryRunes {
		t.Fatalf("country clip: %q", got)
	}
	if got := SanitizeMapQuery("ab\x00c"); got != "abc" {
		t.Fatalf("nul: %q", got)
	}
}

func TestLogsWhereBindsCountryAndQuery(t *testing.T) {
	clause, args := MapScope{Country: "Russia", Query: "tcp"}.LogsWhere()
	if !strings.Contains(clause, "src_country") || !strings.Contains(clause, "positionCaseInsensitiveUTF8") {
		t.Fatalf("clause=%s", clause)
	}
	if strings.Contains(clause, "Russia") || strings.Contains(clause, "tcp") {
		t.Fatal("values must not be interpolated")
	}
	if len(args) != 3 {
		t.Fatalf("args=%d", len(args))
	}
}

func TestLogsWhereAdvancedQueryBinds(t *testing.T) {
	clause, args := MapScope{Query: "country:Germany AND rule:block"}.LogsWhere()
	if !strings.Contains(clause, "AND") || len(args) < 2 {
		t.Fatalf("clause=%s args=%d", clause, len(args))
	}
	if strings.Contains(clause, "Germany") || strings.Contains(clause, "block") {
		t.Fatal("values must not be interpolated")
	}
}

func TestGeoAggHavingIncludesKeys(t *testing.T) {
	expr, args := MapScope{Country: "Germany"}.GeoAggHavingExpr()
	if !strings.Contains(expr, "src_key") || len(args) != 4 {
		t.Fatalf("expr=%s args=%d", expr, len(args))
	}
}

func TestJoinHaving(t *testing.T) {
	if JoinHaving("", "  ") != "" {
		t.Fatal("empty")
	}
	got := JoinHaving(" HAVING sum(blocked_cnt) > 0", "lowerUTF8(src_country) = lowerUTF8(?)")
	if !strings.HasPrefix(got, " HAVING ") || !strings.Contains(got, " AND ") {
		t.Fatalf("%q", got)
	}
}
