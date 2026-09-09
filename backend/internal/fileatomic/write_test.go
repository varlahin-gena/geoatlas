package fileatomic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWriteFileRoundTripAndReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctl.json")
	if err := WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "one" {
		t.Fatalf("got %q", got)
	}
	if err := WriteFile(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "two" {
		t.Fatalf("replace: %q", got)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("tmp file must be gone")
	}
}

func TestWriteJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "doc.json")
	if err := WriteJSON(path, map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{\n  \"n\": 1\n}\n" {
		t.Fatalf("json: %q", got)
	}
}

func TestWriteFileEmptyPath(t *testing.T) {
	if err := WriteFile("  ", []byte("x"), 0o600); err == nil {
		t.Fatal("expected error")
	}
}

// Параллельные писатели в один путь не должны обрезать tmp друг друга:
// файл обязан всегда разбираться как JSON одного из писателей.
func TestWriteJSONConcurrentSamePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctl.json")

	const writers = 8
	const rounds = 25
	payload := strings.Repeat("x", 32<<10) // крупнее буфера ОС, чтобы поймать разрыв

	var wg sync.WaitGroup
	errs := make(chan error, writers*rounds)
	for w := range writers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for r := range rounds {
				doc := map[string]any{"writer": id, "round": r, "pad": payload}
				if err := WriteJSON(path, doc); err != nil {
					errs <- err
					return
				}
				data, err := ReadFile(path)
				if err != nil {
					errs <- err
					return
				}
				var back map[string]any
				if err := json.Unmarshal(data, &back); err != nil {
					errs <- fmt.Errorf("corrupt json after concurrent write (%d bytes): %w", len(data), err)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestSweepStaleTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ctl.json")

	stale := filepath.Join(dir, "ctl.json.tmp-123456")
	legacy := filepath.Join(dir, "ctl.json.tmp")
	fresh := filepath.Join(dir, "ctl.json.tmp-999999")
	other := filepath.Join(dir, "unrelated.json")
	for _, p := range []string{stale, legacy, fresh, other} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * staleTempAge)
	for _, p := range []string{stale, legacy, other} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	if err := WriteJSON(path, map[string]int{"n": 1}); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{stale, legacy} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("stale temp not removed: %s", p)
		}
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh temp of a concurrent writer must survive: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("unrelated file removed: %v", err)
	}
}
