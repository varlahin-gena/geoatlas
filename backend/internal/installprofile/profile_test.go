package installprofile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOKAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	body := `{"profile":"medium","profile_label":"Medium","host":{"cpu_cores":4,"ram_mb":8192},"limits":{"backend":{"ingest_workers":2}},"capacity":{"expected_eps_min":100,"expected_eps_max":500}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil || p.Profile != "medium" || p.Host.CPUCores != 4 {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil {
		t.Fatal("bad json")
	}
}
