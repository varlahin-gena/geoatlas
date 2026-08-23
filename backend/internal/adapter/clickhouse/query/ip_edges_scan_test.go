package query

import (
	"strings"
	"testing"

	"geoatlas/internal/adapter/clickhouse/sqlclause"
)

func TestIPEdgesEnrichOverlaySQLJoinsLookup(t *testing.T) {
	got := ipEdgesEnrichOverlaySQL("SELECT 1")
	for _, want := range []string{
		sqlclause.GeoEnrichIPTable,
		"LEFT JOIN",
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
}
