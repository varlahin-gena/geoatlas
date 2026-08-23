package anomalystore

import (
	"context"
	"strings"
	"testing"
	"time"

	usecaseanomaly "geoatlas/internal/usecase/anomaly"
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

func TestIPNetsSQLSkipsInvalidRange(t *testing.T) {
	clause, args := ipNetsSQL([]string{"src_ip"}, []usecaseanomaly.IPRange{{Start: 20, End: 10}})
	if clause != "" || args != nil {
		t.Fatalf("invalid range: %q %v", clause, args)
	}
}

func TestColInNetsSQLSingleColumn(t *testing.T) {
	clause, args := colInNetsSQL("dst_ip", []usecaseanomaly.IPRange{{Start: 1, End: 2}, {Start: 3, End: 4}})
	if !strings.Contains(clause, "dst_ip") || len(args) != 4 {
		t.Fatalf("clause=%q args=%v", clause, args)
	}
}

func TestRepositoryNilCHGuards(t *testing.T) {
	r := &Repository{}
	ctx := context.Background()
	if m, err := r.ExistingFingerprints(ctx, []string{"fp1"}); err != nil || len(m) != 0 {
		t.Fatalf("ExistingFingerprints: m=%v err=%v", m, err)
	}
	if m, err := r.ActiveSuppressions(ctx, []usecaseanomaly.SuppressionKey{"k"}, time.Now()); err != nil || len(m) != 0 {
		t.Fatalf("ActiveSuppressions: m=%v err=%v", m, err)
	}
	if m, err := r.RecentSuppressionKeys(ctx, usecaseanomaly.CodePortScan, []usecaseanomaly.SuppressionKey{"k"}, time.Now()); err != nil || len(m) != 0 {
		t.Fatalf("RecentSuppressionKeys: m=%v err=%v", m, err)
	}
}

func TestTrafficScannerNilCH(t *testing.T) {
	r := &Repository{}
	ctx := context.Background()
	if _, err := r.OldestLogTime(ctx); err == nil {
		t.Fatal("OldestLogTime want error")
	}
	if m, err := r.KnownPairs(ctx, nil, time.Hour); err != nil || len(m) != 0 {
		t.Fatalf("KnownPairs empty: m=%v err=%v", m, err)
	}
	if _, err := r.CountSummary(ctx, time.Now()); err == nil {
		t.Fatal("CountSummary want error")
	}
}
