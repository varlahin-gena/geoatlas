package sqlclause

import (
	"strings"
	"testing"

	"geoatlas/internal/model"
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
	if !strings.Contains(OrderByAggFilterSQL("all"), "src_ip") {
		t.Fatal("IP edges order must use src_ip:", OrderByAggFilterSQL("all"))
	}
	if !strings.Contains(OrderByAggFilterSQL("all"), "coord_weight DESC") {
		t.Fatal("IP edges order must prefer coord_weight:", OrderByAggFilterSQL("all"))
	}
}

func TestOrderByGeoAggFilterSQL(t *testing.T) {
	got := OrderByGeoAggFilterSQL("all")
	if !strings.HasPrefix(got, "ORDER BY coord_weight DESC") {
		t.Fatalf("coord_weight first: %s", got)
	}
	if !strings.Contains(got, "src_key") || !strings.Contains(got, "dst_key") {
		t.Fatalf("geo daily tables use src_key/dst_key: %s", got)
	}
	if strings.Contains(got, "src_ip") || strings.Contains(got, "dst_ip") {
		t.Fatalf("must not reference src_ip/dst_ip: %s", got)
	}
	blocked := OrderByGeoAggFilterSQL("blocked")
	if !strings.Contains(blocked, "blocked_cnt DESC") || strings.Contains(blocked, "src_ip") {
		t.Fatal(blocked)
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
