package sqlclause

import (
	"strings"
	"testing"

	"network_monitor/internal/model"
)

func TestActionWhereSQL(t *testing.T) {
	if got := ActionWhereSQL("all"); got != "" {
		t.Fatalf("all = %q", got)
	}
	blocked := ActionWhereSQL("blocked")
	if !strings.Contains(blocked, "IN (") || !strings.Contains(blocked, "'deny'") {
		t.Fatalf("blocked where = %q", blocked)
	}
	allowed := ActionWhereSQL("allowed")
	if !strings.Contains(allowed, "NOT IN") {
		t.Fatalf("allowed where = %q", allowed)
	}
}

func TestBlockedAllowedExprsUseModelClause(t *testing.T) {
	in := model.BlockedInClause()
	if !strings.Contains(CountIfBlockedSQL(), in) {
		t.Fatal("countIfBlockedSQL missing BlockedInClause")
	}
	if !strings.Contains(SumAllowedSQL(), in) {
		t.Fatal("sumAllowedSQL missing BlockedInClause")
	}
	if HavingAggFilterSQL("blocked") == "" || HavingAggFilterSQL("") != "" {
		t.Fatal("havingAggFilterSQL unexpected")
	}
}

func TestOrderByAggFilterSQL(t *testing.T) {
	if !strings.Contains(OrderByAggFilterSQL("blocked"), "blocked_cnt DESC") {
		t.Fatal(OrderByAggFilterSQL("blocked"))
	}
	if !strings.Contains(OrderByAggFilterSQL("all"), "cnt DESC") {
		t.Fatal(OrderByAggFilterSQL("all"))
	}
}

func TestOrderByMapAggFilterSQLPrefersGeo(t *testing.T) {
	got := OrderByMapAggFilterSQL("all")
	if !strings.Contains(got, "src_lat != 0") {
		t.Fatalf("expected geo-first order, got %s", got)
	}
	if !strings.HasPrefix(got, "ORDER BY countIf(") {
		t.Fatalf("geo weight should be first via countIf: %s", got)
	}
}
