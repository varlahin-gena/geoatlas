package backupschedulefile

import (
	"os"
	"path/filepath"
	"testing"

	"geoatlas/internal/usecase/backup"
)

func TestNewInvalidSeedAndCorruptLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sched.json")
	// invalid seed → DefaultsSchedule
	store := New(path, backup.Schedule{Keep: -1, Hour: 99})
	got, err := store.Load()
	if err != nil || got.Keep < 1 {
		t.Fatalf("%+v %v", got, err)
	}

	// corrupt JSON with last_run fields preserved via hook
	bad := `{"enabled":true,"hour":99,"minute":0,"timezone":"UTC","keep":7,"last_run_at":"2024-01-01T00:00:00Z","last_run_date":"2024-01-01"}`
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if again.LastRunAt != "2024-01-01T00:00:00Z" || again.LastRunDate != "2024-01-01" {
		t.Fatalf("%+v", again)
	}

	// save invalid rejected
	if err := store.Save(backup.Schedule{Keep: -1, Hour: 3, Minute: 0, Timezone: "UTC"}); err == nil {
		t.Fatal("want save validate err")
	}
}
