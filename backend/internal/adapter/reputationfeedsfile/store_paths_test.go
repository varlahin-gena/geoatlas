package reputationfeedsfile

import (
	"os"
	"path/filepath"
	"testing"

	usecasereputation "geoatlas/internal/usecase/reputation"
)

func TestNormalizeAndNilGuards(t *testing.T) {
	var nilS *Store
	if _, ok, err := nilS.Load(); err != nil || ok {
		t.Fatal()
	}
	if err := nilS.Save(nil); err == nil {
		t.Fatal()
	}
	st := New("  ")
	if _, ok, err := st.Load(); err != nil || ok {
		t.Fatal()
	}
	if err := st.Save(nil); err == nil {
		t.Fatal()
	}

	got := normalizeFeeds([]usecasereputation.Feed{
		{Name: "", URL: "http://x"},
		{Name: "a", URL: ""},
		{Name: " a ", URL: " http://x ", Category: "", Format: ""},
		{Name: "a", URL: "http://dup"}, // duplicate name
		{Name: "b", URL: "http://y", Category: "c2", Format: "CSV_IP"},
	})
	if len(got) != 2 || got[0].Category != "unknown" || got[0].Format != "netset" || got[1].Format != "csv_ip" {
		t.Fatalf("%+v", got)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	st2 := New(path)
	if _, _, err := st2.Load(); err == nil {
		t.Fatal("bad json")
	}
	if _, err := st2.LoadOrSeed(nil); err == nil {
		t.Fatal("loadorseed bad")
	}
	if err := st2.Save(nil); err != nil {
		t.Fatal(err) // empty list ok after normalize
	}
}
