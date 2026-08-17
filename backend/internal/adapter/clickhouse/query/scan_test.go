package query

import (
	"strings"
	"testing"

	"network_monitor/internal/adapter/clickhouse/sqlclause"
)

func TestScanGeoFromLogsSelectClauseOrder(t *testing.T) {
	q := scanGeoFromLogsSelect("traffic_logs", "src_k", "dst_k", "src_l", "dst_l", "timestamp >= now()") +
		"\n\t\t" + limitClause(20000) + AggSettings()

	orderIdx := strings.Index(q, "ORDER BY coord_weight DESC, cnt DESC")
	limitIdx := strings.Index(q, "LIMIT 20000")
	settingsIdx := strings.Index(q, "SETTINGS")
	if orderIdx < 0 || limitIdx < 0 || settingsIdx < 0 {
		t.Fatalf("missing clauses in query:\n%s", q)
	}
	if orderIdx >= limitIdx || limitIdx >= settingsIdx {
		t.Fatalf("expected ORDER BY → LIMIT → SETTINGS, got positions order=%d limit=%d settings=%d\n%s",
			orderIdx, limitIdx, settingsIdx, q)
	}
}

func TestScanGeoFromLogsSelectNoAliasShadowing(t *testing.T) {
	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs("city")
	q := scanGeoFromLogsSelect(
		"traffic_logs",
		srcKey, dstKey,
		srcLabel, dstLabel,
		"timestamp >= now()",
	)
	// Alias matching the column name makes CH nest any(src_city) inside any(labelExpr).
	for _, bad := range []string{
		"any(src_city) AS src_city",
		"any(dst_city) AS dst_city",
		"any(src_country) AS src_country",
		"any(dst_country) AS dst_country",
	} {
		if strings.Contains(q, bad) {
			t.Fatalf("alias shadows column %q in:\n%s", bad, q)
		}
	}
	for _, want := range []string{
		"any(src_city) AS out_src_city",
		"any(dst_city) AS out_dst_city",
		"any(src_country) AS out_src_country",
		"any(dst_country) AS out_dst_country",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in:\n%s", want, q)
		}
	}
}

func TestRawAggSelectSQLNoAliasShadowing(t *testing.T) {
	q := rawAggSelectSQL("traffic_logs", "timestamp >= now()", "all", 100)
	// CH 25: anyIf(src_lon, … src_lat …) AS src_lat + AS src_lon → code 184 nested aggregate.
	for _, bad := range []string{
		"AS src_lat,",
		"AS src_lon,",
		"AS dst_lat,",
		"AS dst_lon,",
		"AS src_city,",
		"AS dst_city,",
	} {
		if strings.Contains(q, bad) {
			t.Fatalf("alias shadows column %q in:\n%s", strings.TrimSuffix(bad, ","), q)
		}
	}
	for _, want := range []string{
		"AS out_src_lat,",
		"AS out_src_lon,",
		"AS out_dst_lat,",
		"AS out_dst_lon,",
		"AS out_src_city,",
		"AS out_dst_city,",
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in:\n%s", want, q)
		}
	}
}

func TestRawAggSelectSQLHasLimitBeforeSettings(t *testing.T) {
	q := rawAggSelectSQL("traffic_logs", "timestamp >= now()", "all", 20000)
	orderIdx := strings.Index(q, "ORDER BY")
	limitIdx := strings.Index(q, "LIMIT 20000")
	settingsIdx := strings.Index(q, "SETTINGS")
	if orderIdx < 0 || limitIdx < 0 || settingsIdx < 0 {
		t.Fatalf("missing clauses:\n%s", q)
	}
	if orderIdx >= limitIdx || limitIdx >= settingsIdx {
		t.Fatalf("expected ORDER→LIMIT→SETTINGS, got %d %d %d", orderIdx, limitIdx, settingsIdx)
	}
}
