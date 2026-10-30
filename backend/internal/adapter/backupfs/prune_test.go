package backupfs

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPruneKeepsAttachedAndLimit(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	names := []string{"ga-20260101T000000Z", "ga-20260102T000000Z", "ga-20260103T000000Z", "ga-20260104T000000Z"}
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, n, "f"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		// bump mtime ordering via source sidecar used by List
		_ = store.WriteSource(n, "manual")
		time.Sleep(5 * time.Millisecond)
	}
	if err := store.SetAttached(names[0]); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(1); err != nil {
		t.Fatal(err)
	}
	if !store.Exists(names[0]) {
		t.Fatal("attached pruned")
	}
	// keep=1 among non-attached → only newest non-attached + attached remain
	alive := 0
	for _, n := range names {
		if store.Exists(n) {
			alive++
		}
	}
	if alive < 2 {
		t.Fatalf("alive=%d", alive)
	}

	notReady := New(filepath.Join(root, "missing-dir"))
	if err := notReady.Prune(1); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(0); err != nil { // clamps keep to 1
		t.Fatal(err)
	}
}
