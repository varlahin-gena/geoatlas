package anomalystore

import (
	"strings"
	"testing"

	usecaseanomaly "network_monitor/internal/usecase/anomaly"
)

func TestTouchNetsSQLEmpty(t *testing.T) {
	clause, args := touchNetsSQL(nil)
	if clause != "" || args != nil {
		t.Fatalf("empty nets: %q %v", clause, args)
	}
}

func TestTouchNetsSQLRange(t *testing.T) {
	clause, args := touchNetsSQL([]usecaseanomaly.IPRange{{Start: 10, End: 20}})
	if !strings.Contains(clause, "src_ip") || !strings.Contains(clause, "dst_ip") {
		t.Fatalf("clause: %s", clause)
	}
	if len(args) != 4 {
		t.Fatalf("args=%v", args)
	}
}
