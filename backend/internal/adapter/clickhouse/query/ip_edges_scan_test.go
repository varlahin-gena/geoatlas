package query

import (
	"strings"
	"testing"

	"geoatlas/internal/adapter/clickhouse/sqlclause"
)

func TestIPEdgesEnrichOverlaySQLJoinsLookup(t *testing.T) {
	got := ipEdgesEnrichOverlaySQL("SELECT 1", true)
	for _, want := range []string{
		sqlclause.GeoEnrichIPTable,
		"ANY LEFT JOIN",
		"AS sg ON e.src_ip = sg.ip",
		"AS dg ON e.dst_ip = dg.ip",
		"sg.city",
		"dg.city",
		"sg.lat",
		"dg.lat",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	plain := ipEdgesEnrichOverlaySQL("SELECT 1", false)
	if strings.Contains(plain, sqlclause.GeoEnrichIPTable) {
		t.Fatalf("overlay off must skip enrich:\n%s", plain)
	}
	if !strings.Contains(plain, "(SELECT 1)") {
		t.Fatalf("overlay off must keep inner:\n%s", plain)
	}
}
