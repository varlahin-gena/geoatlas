package anomalysettingsfile

import (
	"os"
	"path/filepath"
	"testing"

	usecaseanomaly "geoatlas/internal/usecase/anomaly"
)

func TestStoreRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "anomaly_settings.json")
	s := New(path)

	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !isEmptySettings(got) {
		t.Fatalf("empty file: %+v", got)
	}

	want := usecaseanomaly.Settings{
		Enabled:            true,
		ScanIntervalMin:    10,
		LearningDays:       5,
		SuppressHours:      24,
		IncludePrivate:     false,
		NewCountryMinShare: 0.05,
		UpdatedAt:          "2026-01-01T00:00:00Z",
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

func isEmptySettings(st usecaseanomaly.Settings) bool {
	return !st.Enabled &&
		st.ScanIntervalMin == 0 &&
		st.LearningDays == 0 &&
		st.SuppressHours == 0 &&
		!st.IncludePrivate &&
		st.NewCountryMinShare == 0 &&
		st.UpdatedAt == ""
}
