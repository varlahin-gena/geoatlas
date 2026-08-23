package backupschedulefile

import (
	"path/filepath"
	"testing"

	"geoatlas/internal/usecase/backup"
)

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup_schedule.json")
	seed := backup.DefaultsSchedule(backup.Options{Keep: 5, IncludeEdges: true, IncludeAuth: false})
	store := New(path, seed)

	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Keep != 5 || got.Enabled {
		t.Fatalf("seed load: %+v", got)
	}

	got.Enabled = true
	got.Hour = 4
	got.Minute = 15
	got.Timezone = "UTC"
	got.Keep = 10
	got.UpdatedAt = "2026-01-01T00:00:00Z"
	if err := store.Save(got); err != nil {
		t.Fatal(err)
	}
	again, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !again.Enabled || again.Hour != 4 || again.Minute != 15 || again.Keep != 10 {
		t.Fatalf("reload: %+v", again)
	}
}
