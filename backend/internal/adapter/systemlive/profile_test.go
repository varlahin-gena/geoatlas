package systemlive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProfileAdapterLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	body := `{"generated_at":"t","profile":"small","profile_label":"Small","host":{"cpu_cores":2,"ram_mb":4096,"disk_gb_avail":100,"cgroup":"x"},"limits":{"clickhouse":{"memory_gb":2,"cpus":1,"max_query_memory_bytes":1,"external_spill_bytes":2},"backend":{"memory_gb":1,"cpus":1,"ingest_workers":2,"ingest_queue_size":100,"ingest_batch_size":50},"syslog_ng":{"memory_mb":128,"cpus":1,"fifo_size":1,"mem_buf_bytes":2,"disk_buf_bytes":3,"udp_rcvbuf_bytes":4,"iw_size":5,"tcp_iw_size":6,"tcp_max_conn":7}},"capacity":{"expected_eps_min":10,"expected_eps_max":20}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := (ProfileAdapter{}).Load(path)
	if err != nil || p.Profile != "small" || p.Host.CPUCores != 2 || p.Limits.Backend.IngestWorkers != 2 {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := (ProfileAdapter{}).Load(filepath.Join(dir, "missing.json")); err == nil {
		t.Fatal("missing")
	}
}
