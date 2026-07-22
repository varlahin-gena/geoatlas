package system

import (
	"time"

	"network_monitor/internal/installprofile"
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
	Backend        BackendInfo                   `json:"backend_info"`
	InstallProfile *installprofile.Profile       `json:"install_profile,omitempty"`
	EdgesAgg       EdgesAggView                  `json:"edges_agg"`
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
	GoVersion    string  `json:"go_version"`
	NumGoroutine int     `json:"num_goroutine"`
	HeapAllocMB  float64 `json:"heap_alloc_mb"`
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
