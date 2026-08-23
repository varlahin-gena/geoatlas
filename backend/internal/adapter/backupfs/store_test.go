package backupfs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAuthTarballSkipsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	backups := filepath.Join(root, "backups")
	outside := filepath.Join(root, "outside.txt")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "users.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(data, "leak")
	if err := os.Symlink(outside, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	store := New(backups)
	name := "ga-20260101T000000Z"
	if err := store.WriteAuthTarball(name, data); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(filepath.Join(backups, name+".auth.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Name == "leak" || filepath.Base(hdr.Name) == "outside.txt" {
			t.Fatalf("symlink target leaked into tar: %s", hdr.Name)
		}
		body, _ := io.ReadAll(tr)
		if string(body) == "secret" {
			t.Fatal("outside file content present in tar")
		}
	}
}

func TestWriteAuthTarballSkipsSnapLockTmp(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "data")
	backups := filepath.Join(root, "backups")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "users.json"), []byte(`{"ok":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "geo_index.snap"), []byte("SNAP"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "users.json.tmp"), []byte("tmp"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, ".ga_backend.lock"), []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	store := New(backups)
	name := "ga-20260101T000000Z"
	if err := store.WriteAuthTarball(name, data); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(backups, name+".auth.tgz"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	seenUsers := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		base := filepath.Base(hdr.Name)
		switch base {
		case "geo_index.snap", "users.json.tmp", ".ga_backend.lock":
			t.Fatalf("control-plane cache leaked into auth tarball: %s", hdr.Name)
		case "users.json":
			seenUsers = true
		}
	}
	if !seenUsers {
		t.Fatal("users.json missing from auth tarball")
	}
}

func TestParseBackupNameTimeOffset(t *testing.T) {
	tUTC, ok := parseBackupNameTime("ga-20260811T072119Z")
	if !ok || !tUTC.Equal(time.Date(2026, 8, 11, 7, 21, 19, 0, time.UTC)) {
		t.Fatalf("utc: ok=%v t=%v", ok, tUTC)
	}
	tOff, ok := parseBackupNameTime("ga-20260811T102119+0300")
	if !ok || !tOff.Equal(time.Date(2026, 8, 11, 7, 21, 19, 0, time.UTC)) {
		t.Fatalf("offset: ok=%v t=%v", ok, tOff)
	}
	if !nameRe.MatchString("ga-20260811T102119+0300") {
		t.Fatal("regex should accept ga offset name")
	}
	if !nameRe.MatchString("ga-20260811T072119Z") {
		t.Fatal("regex should accept ga UTC name")
	}
}

func TestWriteSourceAndList(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	name := "ga-20260101T120000Z"
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSource(name, "manual"); err != nil {
		t.Fatal(err)
	}
	list, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Source != "manual" {
		t.Fatalf("got %+v", list)
	}
	if err := store.Delete(name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, name+".source")); !os.IsNotExist(err) {
		t.Fatalf("source marker should be removed: %v", err)
	}
}

func TestAttachedMarkerAndDelete(t *testing.T) {
	root := t.TempDir()
	store := New(root)
	name := "ga-20260101T000000Z"
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatal(err)
	}
	if !store.Exists(name) {
		t.Fatal("expected exists")
	}
	if err := store.SetAttached(name); err != nil {
		t.Fatal(err)
	}
	got, err := store.Attached()
	if err != nil || got != name {
		t.Fatalf("attached=%q err=%v", got, err)
	}
	if err := store.SetAttached(""); err != nil {
		t.Fatal(err)
	}
	got, err = store.Attached()
	if err != nil || got != "" {
		t.Fatalf("cleared attached=%q err=%v", got, err)
	}
	if err := store.Delete(name); err != nil {
		t.Fatal(err)
	}
	if store.Exists(name) {
		t.Fatal("expected deleted")
	}
}
