package query

import (
	"strings"
	"testing"
)

func TestGeoMissingSQLHasLimitAndSettings(t *testing.T) {
	// sanity: helper pieces used by ScanGeoMissingIPsForTimeRange
	frag := limitClause(500) + AggSettings()
	if !strings.Contains(frag, "LIMIT 500") {
		t.Fatalf("missing LIMIT: %q", frag)
	}
	if !strings.Contains(frag, "max_memory_usage") {
		t.Fatalf("missing SETTINGS: %q", frag)
	}
}
