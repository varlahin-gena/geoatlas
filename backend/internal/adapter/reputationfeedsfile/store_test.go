package reputationfeedsfile

import (
	"os"
	"path/filepath"
	"testing"

	usecasereputation "network_monitor/internal/usecase/reputation"
)

func TestLoadOrSeedAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "feeds.json")
	st := New(path)

	seed := []usecasereputation.Feed{{
		Name: "a", URL: "https://example.com/a.netset", Category: "attacks", Format: "netset",
	}}
	got, err := st.LoadOrSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "a" {
		t.Fatalf("%+v", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}

	// повторный LoadOrSeed не перетирает
	got2, err := st.LoadOrSeed([]usecasereputation.Feed{{
		Name: "b", URL: "https://example.com/b.netset", Category: "c2", Format: "netset",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0].Name != "a" {
		t.Fatalf("should keep seeded file: %+v", got2)
	}

	if err := st.Save([]usecasereputation.Feed{
		{Name: "x", URL: "https://example.com/x", Category: "drop", Format: "netset"},
		{Name: "y", URL: "https://example.com/y", Category: "c2", Format: "netset"},
	}); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := st.Load()
	if err != nil || !ok || len(loaded) != 2 {
		t.Fatalf("load after save: ok=%v n=%d err=%v", ok, len(loaded), err)
	}
}
