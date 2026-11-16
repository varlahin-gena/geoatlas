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

func TestLoadFractionalMemory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	body := `{"profile":"tiny","limits":{"clickhouse":{"memory_gb":1.5},"backend":{"memory_gb":0.75,"ingest_queue_max_bytes":67108864}}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Limits.ClickHouse.MemoryGB != 1.5 || p.Limits.Backend.MemoryGB != 0.75 {
		t.Fatalf("memory: ch=%v be=%v", p.Limits.ClickHouse.MemoryGB, p.Limits.Backend.MemoryGB)
	}
	if p.Limits.Backend.IngestQueueMaxBytes != 67_108_864 {
		t.Fatalf("queue bytes: %d", p.Limits.Backend.IngestQueueMaxBytes)
	}
}
