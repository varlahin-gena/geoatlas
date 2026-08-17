package fileatomic

import (
	"os"
	"path/filepath"
	"testing"
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
