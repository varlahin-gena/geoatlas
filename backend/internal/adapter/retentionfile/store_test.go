package retentionfile

import (
	"os"
	"path/filepath"
	"testing"

	"geoatlas/internal/usecase/retention"
)

func TestStoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "retention.json")
	s := New(path)

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	def := retention.Defaults()
	if got.TrafficLogsDays != def.TrafficLogsDays {
		t.Fatalf("empty file defaults: %+v", got)
	}

	want := retention.Settings{
		TrafficLogsDays: 10, EdgesDays: 20, ParseErrorsDays: 3, SystemMetricsDays: 4,
		UpdatedAt: "2026-01-01T00:00:00Z",
	}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v want %+v", got, want)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
