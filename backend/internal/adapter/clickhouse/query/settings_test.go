package query

import (
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
