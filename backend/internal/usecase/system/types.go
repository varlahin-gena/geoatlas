package system

import (
	"time"

	"network_monitor/internal/model"
)

type SystemStatsResponse struct {
	Timestamp      string                        `json:"timestamp"`
	UptimeSec      float64                       `json:"uptime_sec"`
	Containers     map[string]map[string]float64 `json:"containers"`
	Pipeline       map[string]map[string]float64 `json:"pipeline"`
	Storage        map[string]map[string]float64 `json:"storage"`
	Health         map[string]map[string]any     `json:"health"`
	Alerts         []Alert                       `json:"alerts"`
	IngestSLO      IngestSLO                     `json:"ingest_slo"`
	Backend        BackendInfo                   `json:"backend_info"`
	InstallProfile *CapacityProfile              `json:"install_profile,omitempty"`
	EdgesAgg       EdgesAggView                  `json:"edges_agg"`
}

// CapacityProfile is the installation sizing data required by system APIs.
// It mirrors the external install-profile document without coupling the use case
// to its file-format package.
type CapacityProfile struct {
	GeneratedAt  string          `json:"generated_at"`
	Host         ProfileHost     `json:"host"`
	Profile      string          `json:"profile"`
	ProfileLabel string          `json:"profile_label"`
	Limits       ProfileLimits   `json:"limits"`
	Capacity     ProfileCapacity `json:"capacity"`
}

// InstallMeta is written by start.sh (deploy/common/install_meta.sh).
type InstallMeta struct {
	Version string `json:"version"`
	Source  string `json:"source"`
	Ref     string `json:"ref"`
	Commit  string `json:"commit,omitempty"`
	Display string `json:"display"`
}

type ProfileHost struct {
	CPUCores    int    `json:"cpu_cores"`
	RAMMB       int    `json:"ram_mb"`
	DiskGBAvail int    `json:"disk_gb_avail"`
	Cgroup      string `json:"cgroup"`
}

type ProfileServiceLimits struct {
	MemoryGB int `json:"memory_gb"`
	CPUs     int `json:"cpus"`
}

type ProfileClickHouseLimits struct {
	ProfileServiceLimits
	MaxQueryMemoryBytes int64 `json:"max_query_memory_bytes"`
	ExternalSpillBytes  int64 `json:"external_spill_bytes"`
}

type ProfileBackendLimits struct {
	ProfileServiceLimits
	IngestWorkers   int `json:"ingest_workers"`
	IngestQueueSize int `json:"ingest_queue_size"`
	IngestBatchSize int `json:"ingest_batch_size"`
}

type ProfileSyslogLimits struct {
	MemoryMB       int   `json:"memory_mb"`
	CPUs           int   `json:"cpus"`
	FifoSize       int   `json:"fifo_size,omitempty"`
	MemBufBytes    int64 `json:"mem_buf_bytes,omitempty"`
	DiskBufBytes   int64 `json:"disk_buf_bytes,omitempty"`
	UDPRcvbufBytes int64 `json:"udp_rcvbuf_bytes,omitempty"`
	IWSize         int   `json:"iw_size,omitempty"`
}

type ProfileLimits struct {
	ClickHouse ProfileClickHouseLimits `json:"clickhouse"`
	Backend    ProfileBackendLimits    `json:"backend"`
	SyslogNG   ProfileSyslogLimits     `json:"syslog_ng"`
}

type ProfileCapacity struct {
	ExpectedEPSMin int `json:"expected_eps_min"`
	ExpectedEPSMax int `json:"expected_eps_max"`
}

// SystemStatusResponse is the compact status for authenticated indicators.
type SystemStatusResponse struct {
	Level      string  `json:"level"`
	AlertCount int     `json:"alert_count"`
	Alerts     []Alert `json:"alerts"`
}

// EdgesAggView is aggregate maintenance state and map-source guidance.
type EdgesAggView struct {
	State        string    `json:"state"`
	Phase        string    `json:"phase,omitempty"`
	Message      string    `json:"message"`
	RawRows      uint64    `json:"raw_rows"`
	AggRows      uint64    `json:"agg_rows"`
	DaysTotal    int       `json:"days_total"`
	DaysDone     int       `json:"days_done"`
	StartedAt    time.Time `json:"started_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	PreferAgg    bool      `json:"prefer_agg"`
	GeoPreferAgg bool      `json:"geo_prefer_agg"`
	MapSource    string    `json:"map_source"`
}

type Alert struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

type BackendInfo struct {
	GoVersion      string  `json:"go_version"`
	NumGoroutine   int     `json:"num_goroutine"`
	HeapAllocMB    float64 `json:"heap_alloc_mb"`
	GeoIndexRanges int     `json:"geo_index_ranges,omitempty"`
	GeoIndexMB     float64 `json:"geo_index_mb,omitempty"`
}

type HistoryResponse struct {
	Period  string                          `json:"period"`
	StepSec int                             `json:"step_sec"`
	From    string                          `json:"from"`
	To      string                          `json:"to"`
	Series  map[string][]model.HistoryPoint `json:"series"`
}

// HealthResult is an HTTP-ready health result.
type HealthResult struct {
	OK         bool
	HTTPStatus int
	Body       map[string]any
}
