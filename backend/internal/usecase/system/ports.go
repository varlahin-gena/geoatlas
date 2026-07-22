package system

import (
	"context"
	"time"

	"network_monitor/internal/installprofile"
	"network_monitor/internal/model"
)

// MetricsStore provides persisted system metrics.
type MetricsStore interface {
	FetchLatest(ctx context.Context) ([]model.MetricRecord, error)
	FetchHistory(ctx context.Context, keys []model.MetricKey, period, step time.Duration) (map[string][]model.HistoryPoint, error)
	CountRows(ctx context.Context, table string) (uint64, error)
}

// EdgesAggReader provides the current edges aggregate state.
type EdgesAggReader interface {
	Status() EdgesAggStatus
	PreferDaily() bool
	PreferGeo() bool
}

// EdgesAggStatus is the aggregate maintenance state required by this use case.
type EdgesAggStatus struct {
	State, Phase, Message string
	RawRows, AggRows      uint64
	DaysTotal, DaysDone   int
	StartedAt, UpdatedAt  time.Time
}

// IngestSnapshot is the live ingest data required for monitoring.
type IngestSnapshot struct {
	State                                                                     string
	ReceivedTotal, ParsedTotal, InsertedTotal, SkippedTotal, ParseErrorsTotal int64
	BufferedLines, QueueDepth, QueueCapacity, DroppedTotal, Connections       int64
	UDPReceived, UDPConnections, TCPReceived, TCPConnections                  int64
	LastError                                                                 string
}

// IngestLive exposes optional live ingest metrics.
type IngestLive interface {
	// Snapshot returns ok=false when ingest is disabled.
	Snapshot() (IngestSnapshot, bool)
}

// ClickHousePinger checks ClickHouse liveness.
type ClickHousePinger interface {
	Ping(ctx context.Context) error
}

// ProfileLoader loads the installation capacity profile.
type ProfileLoader interface {
	Load(path string) (*installprofile.Profile, error)
}

// MaintenanceScheduler — сериализованный edges/geo backfill (реализация: *geojob.Scheduler).
type MaintenanceScheduler interface {
	ScheduleMaintenanceBackfill(parent context.Context, timeout time.Duration)
}
