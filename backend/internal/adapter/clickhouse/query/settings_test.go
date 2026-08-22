package query

import (
	"fmt"
	"strings"
	"testing"
)

func TestAggSettingsIncludesMaxThreads(t *testing.T) {
	ConfigureQuerySettings(1<<30, 64<<20, 64<<20, 3)
	t.Cleanup(func() {
		ConfigureQuerySettings(2<<30, 256<<20, 256<<20, 2)
	})
	s := AggSettings()
	for _, want := range []string{
		"max_memory_usage = 1073741824",
		"max_threads = 3",
		"max_bytes_before_external_group_by = 67108864",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in:\n%s", want, s)
		}
	}
}

func TestConfigureQuerySettingsKeepsDefaultThreads(t *testing.T) {
	ConfigureQuerySettings(2<<30, 256<<20, 256<<20, 0)
	t.Cleanup(func() {
		ConfigureQuerySettings(2<<30, 256<<20, 256<<20, 2)
	})
	if !strings.Contains(AggSettings(), "max_threads = 2") {
		t.Fatalf("expected default max_threads=2, got:\n%s", AggSettings())
	}
}

func TestAggSettingsSpillBelowMemoryHeadroom(t *testing.T) {
	// 1.2 GiB query cap (typical 3 GiB CH container @ 40%) with high spill request.
	ConfigureQuerySettings(1288490188, 512<<20, 512<<20, 2)
	t.Cleanup(func() {
		ConfigureQuerySettings(2<<30, 256<<20, 256<<20, 2)
	})
	s := AggSettings()
	if !strings.Contains(s, "max_memory_usage = 1288490188") {
		t.Fatalf("missing memory cap:\n%s", s)
	}
	// Headroom = max/4 ≈ 322MB; spill must be capped below the old 512MB request.
	if strings.Contains(s, "max_bytes_before_external_group_by = 536870912") {
		t.Fatalf("spill not capped for headroom:\n%s", s)
	}
	if !strings.Contains(s, "max_bytes_before_external_group_by = 322122547") {
		t.Fatalf("expected spill=max/4, got:\n%s", s)
	}
}

func TestBackfillAggSettingsUsesSingleThread(t *testing.T) {
	ConfigureQuerySettings(1288490188, 512<<20, 512<<20, 4)
	t.Cleanup(func() {
		ConfigureQuerySettings(2<<30, 256<<20, 256<<20, 2)
	})
	s := BackfillAggSettings()
	if !strings.Contains(s, "max_threads = 1") {
		t.Fatalf("expected max_threads=1:\n%s", s)
	}
	wantSpill := 1288490188 / 16
	if !strings.Contains(s, fmt.Sprintf("max_bytes_before_external_group_by = %d", wantSpill)) {
		t.Fatalf("expected spill=max/16 (%d):\n%s", wantSpill, s)
	}
}
