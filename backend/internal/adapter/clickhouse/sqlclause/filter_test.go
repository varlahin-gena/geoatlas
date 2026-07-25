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

func TestActionWhereSQLRejectsUnknown(t *testing.T) {
	// Только точные "blocked"/"allowed"; всё остальное — без WHERE (all).
	for _, f := range []string{"", "all", "BLOCKED", "x' OR 1=1", "blocked;--", "allowed "} {
		if got := ActionWhereSQL(f); got != "" {
			t.Fatalf("ActionWhereSQL(%q)=%q want empty", f, got)
		}
	}
	if ActionWhereSQL("blocked") == "" || ActionWhereSQL("allowed") == "" {
		t.Fatal("blocked/allowed must produce SQL")
	}
}
