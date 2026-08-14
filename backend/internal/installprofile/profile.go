package installprofile

import (
	"encoding/json"
	"os"
	"time"
)

type HostInfo struct {
	CPUCores    int    `json:"cpu_cores"`
	RAMMB       int    `json:"ram_mb"`
	DiskGBAvail int    `json:"disk_gb_avail"`
	Cgroup      string `json:"cgroup"`
}

type ServiceLimits struct {
	MemoryGB int `json:"memory_gb"`
	CPUs     int `json:"cpus"`
}

type ClickHouseLimits struct {
	ServiceLimits
	MaxQueryMemoryBytes int64 `json:"max_query_memory_bytes"`
	ExternalSpillBytes  int64 `json:"external_spill_bytes"`
}

type BackendLimits struct {
	ServiceLimits
	IngestWorkers   int `json:"ingest_workers"`
	IngestQueueSize int `json:"ingest_queue_size"`
	IngestBatchSize int `json:"ingest_batch_size"`
}

type SyslogLimits struct {
	MemoryMB       int   `json:"memory_mb"`
	CPUs           int   `json:"cpus"`
	FifoSize       int   `json:"fifo_size,omitempty"`
	MemBufBytes    int64 `json:"mem_buf_bytes,omitempty"`
	DiskBufBytes   int64 `json:"disk_buf_bytes,omitempty"`
	UDPRcvbufBytes int64 `json:"udp_rcvbuf_bytes,omitempty"`
	IWSize         int   `json:"iw_size,omitempty"`
}

type Limits struct {
	ClickHouse ClickHouseLimits `json:"clickhouse"`
	Backend    BackendLimits    `json:"backend"`
	SyslogNG   SyslogLimits     `json:"syslog_ng"`
}

type Capacity struct {
	ExpectedEPSMin int `json:"expected_eps_min"`
	ExpectedEPSMax int `json:"expected_eps_max"`
}

type Profile struct {
	GeneratedAt  string   `json:"generated_at"`
	Host         HostInfo `json:"host"`
	Profile      string   `json:"profile"`
	ProfileLabel string   `json:"profile_label"`
	Limits       Limits   `json:"limits"`
	Capacity     Capacity `json:"capacity"`
}

func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func GeneratedAtTime(p *Profile) (time.Time, bool) {
	if p == nil || p.GeneratedAt == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, p.GeneratedAt)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
