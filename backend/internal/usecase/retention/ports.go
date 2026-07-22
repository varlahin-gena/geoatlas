package retention

import "context"

// Settings — желаемые TTL (дни) для групп таблиц.
type Settings struct {
	TrafficLogsDays   int    `json:"traffic_logs_days"`
	EdgesDays         int    `json:"edges_days"`
	ParseErrorsDays   int    `json:"parse_errors_days"`
	SystemMetricsDays int    `json:"system_metrics_days"`
	UpdatedAt         string `json:"updated_at,omitempty"`
}

// Store — персистентность настроек (файл на диске).
type Store interface {
	Load() (Settings, error)
	Save(s Settings) error
}

// TTLApplier — MODIFY TTL в ClickHouse.
type TTLApplier interface {
	ApplyTrafficLogs(ctx context.Context, days int) error
	ApplyEdges(ctx context.Context, days int) error
	ApplyParseErrors(ctx context.Context, days int) error
	ApplySystemMetrics(ctx context.Context, days int) error
}
