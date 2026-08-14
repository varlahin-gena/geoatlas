package system

import (
	"context"
	"time"

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
	BufferDropsTotal                                                          int64
	QueueBytes, QueueBytesCapacity                                            int64
	UDPReceived, UDPConnections, TCPReceived, TCPConnections                  int64
	CircuitOpen                                                               bool
	LastError                                                                 string
	LastDropAt                                                                string // RFC3339; empty if never dropped
}

// SyslogNGSnapshot is live stats-exporter data (drops/queue before backend ingest).
type SyslogNGSnapshot struct {
	Up             bool
	DroppedTotal   int64
	Queued         int64
	ProcessedTotal int64
	UDPProcessed   int64
	TCPProcessed   int64
}

// IngestLive exposes optional live ingest metrics.
type IngestLive interface {
	// Snapshot returns ok=false when ingest is disabled.
	Snapshot() (IngestSnapshot, bool)
}

// SyslogNGLive scrapes syslog-ng stats-exporter. ok=false when URL is not configured.
type SyslogNGLive interface {
	Snapshot(ctx context.Context) (SyslogNGSnapshot, bool)
}

// GeoIndexLive exposes compact GeoIP index size for observability.
type GeoIndexLive interface {
	RangeCount() int
	ApproxBytes() uint64
}

// ClickHousePinger checks ClickHouse liveness.
type ClickHousePinger interface {
	Ping(ctx context.Context) error
}

// ProfileLoader loads the installation capacity profile.
type ProfileLoader interface {
	Load(path string) (*CapacityProfile, error)
}

// MaintenanceScheduler — сериализованный edges/geo backfill (реализация: *geojob.Scheduler).
type MaintenanceScheduler interface {
	ScheduleMaintenanceBackfill(parent context.Context, timeout time.Duration)
}
