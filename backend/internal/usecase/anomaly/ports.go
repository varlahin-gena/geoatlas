package anomaly

import (
	"context"
	"time"

	"network_monitor/internal/model"
)

// EventStore — журнал аномалий.
type EventStore interface {
	Insert(ctx context.Context, events []Event) error
	List(ctx context.Context, q ListQuery) ([]Event, error)
	ExistingFingerprints(ctx context.Context, fps []string) (map[string]struct{}, error)
	Ack(ctx context.Context, fingerprint, by string) error
	CountSummary(ctx context.Context, since time.Time) (Summary, error)
}

// TrafficScanner — SQL-сканы для детекторов (без CH-типов в usecase).
type TrafficScanner interface {
	OldestLogTime(ctx context.Context) (time.Time, error)
	PortScan(ctx context.Context, window time.Duration, portsTh, eventsTh int, includePrivate bool) ([]PortScanHit, error)
	HorizontalScan(ctx context.Context, window time.Duration, hostsTh, eventsTh int, includePrivate bool) ([]HorizontalScanHit, error)
	BlockedCount(ctx context.Context, start, end time.Time) (uint64, error)
	CurrentCountries(ctx context.Context, window time.Duration, minN uint64) ([]CountryCount, error)
	BaselineCountries(ctx context.Context, days int, minN uint64) (map[string]struct{}, error)
	RecentEdges(ctx context.Context, window time.Duration, limit int) ([]EdgeRow, error)
	KnownPairs(ctx context.Context, pairs [][2]string, lookback time.Duration) (map[string]struct{}, error)
}

// ReputationLookuper — in-memory reputation (optional).
type ReputationLookuper interface {
	Lookup(ip string) []model.ReputationHit
}

// Gate — пропуск тика (circuit / rebuild). Пустая строка = работать.
type Gate interface {
	SkipReason() string
}

// ProfileName — имя install profile (tiny…xlarge).
type ProfileName func() string

// Metrics — Prometheus.
type Metrics interface {
	ObserveScan(d time.Duration, inserted int, skipReason string)
	IncDetected(code, severity string)
	IncScanError(code string)
}
