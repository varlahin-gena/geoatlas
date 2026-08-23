package geostore

import (
	"os"
	"path/filepath"
	"testing"

	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
)

func TestReloadableGeoIndexDiskRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geo_index.snap")
	idx := NewReloadableGeoIndex(nil, path)
	b := geoip.NewCompactBuilder(2)
	b.AddRange(model.GeoRange{
		StartIP: 1, EndIP: 10, Country: "RU", City: "Moscow", Lat: 55.75, Lon: 37.62,
	})
	idx.ReplaceBuiltSnapshot(b.Build())
	if !idx.IndexReady() {
		t.Fatal("ready after persist")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("expected snapshot file", err)
	}

	idx2 := NewReloadableGeoIndex(nil, path)
	if !idx2.LoadDisk() {
		t.Fatal("LoadDisk")
	}
	if idx2.RangeCount() != 1 {
		t.Fatalf("count=%d", idx2.RangeCount())
	}
	lu := idx2.Lookup("0.0.0.5")
	if !lu.Found || lu.City != "Moscow" {
		t.Fatalf("lookup %+v", lu)
	}

	idx2.ReplaceRanges(nil)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("clear must remove snapshot: %v", err)
	}
}

func TestReloadableGeoIndexLoadDiskMissing(t *testing.T) {
	idx := NewReloadableGeoIndex(nil, filepath.Join(t.TempDir(), "missing.snap"))
	if idx.LoadDisk() {
		t.Fatal("missing file")
	}
	if idx.IndexReady() {
		t.Fatal("must not be ready")
	}
}
