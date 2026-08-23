package migrate

import (
	"strings"
	"testing"

	"geoatlas/internal/adapter/clickhouse/sqlclause"
)

func TestGeoEdgesAggSelectBodyNoAliasShadowing(t *testing.T) {
	for _, groupBy := range []string{"city", "country"} {
		srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprsPrefixed("traffic_logs", groupBy)
		body := geoEdgesAggSelectBody(srcKey, dstKey, srcLabel, dstLabel, sqlclause.GeoCoordOK)
		for _, bad := range []string{
			"anyState(src_city) AS src_city",
			"anyState(dst_city) AS dst_city",
			"anyState(src_country) AS src_country",
			"anyState(dst_country) AS dst_country",
			"trimBoth(src_city)",
			"trimBoth(dst_city)",
			"trimBoth(src_country)",
			"trimBoth(dst_country)",
		} {
			if strings.Contains(body, bad) {
				t.Fatalf("group_by=%s: alias-shadowing fragment %q in:\n%s", groupBy, bad, body)
			}
		}
		for _, want := range []string{
			"anyState(traffic_logs.src_city) AS src_city",
			"anyState(traffic_logs.dst_city) AS dst_city",
			"anyState(traffic_logs.src_country) AS src_country",
			"anyState(traffic_logs.dst_country) AS dst_country",
			"traffic_logs.",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("group_by=%s: missing %q in:\n%s", groupBy, want, body)
			}
		}
	}
}

func TestGeoEdgesEnrichedFromSQLJoinsLookup(t *testing.T) {
	pred := sqlclause.DayTimestampRangeSQL("traffic_logs.timestamp")
	from := geoEdgesEnrichedFromSQL(pred)
	for _, want := range []string{
		sqlclause.GeoEnrichIPTable,
		"LEFT JOIN",
		"AS sg ON",
		"AS dg ON",
		"AS traffic_logs",
		pred,
	} {
		if !strings.Contains(from, want) {
			t.Fatalf("enriched FROM missing %q:\n%s", want, from)
		}
	}
	if strings.Contains(from, "ALTER TABLE") {
		t.Fatalf("enriched FROM must not contain ALTER TABLE:\n%s", from)
	}
	if strings.Contains(from, "toDate(traffic_logs.timestamp) =") {
		t.Fatalf("enriched FROM must use timestamp range prune, not toDate()=:\n%s", from)
	}
}
