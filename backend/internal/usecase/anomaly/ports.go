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
	ActiveSuppressions(ctx context.Context, keys []SuppressionKey, now time.Time) (map[SuppressionKey]struct{}, error)
	RecentSuppressionKeys(ctx context.Context, code string, keys []SuppressionKey, since time.Time) (map[SuppressionKey]struct{}, error)
	Ack(ctx context.Context, fingerprint, by string, suppressFor time.Duration) error
	CountSummary(ctx context.Context, since time.Time) (Summary, error)
}

// EnterpriseNetSource — отмеченные подсети предприятия.
type EnterpriseNetSource interface {
	ListEnterpriseNets(ctx context.Context) ([]model.EnterpriseNet, error)
}

// TrafficScanner — SQL-сканы для детекторов (без CH-типов в usecase).
type TrafficScanner interface {
	OldestLogTime(ctx context.Context) (time.Time, error)
	PortScan(ctx context.Context, window time.Duration, portsTh, eventsTh int, includePrivate bool, nets []IPRange) ([]PortScanHit, error)
	HorizontalScan(ctx context.Context, window time.Duration, hostsTh, eventsTh int, includePrivate bool, nets []IPRange) ([]HorizontalScanHit, error)
	BlockedCount(ctx context.Context, start, end time.Time, net *IPRange) (uint64, error)
	CurrentCountries(ctx context.Context, window time.Duration, minN uint64, nets []IPRange) ([]CountryCount, error)
	CurrentCountryTotal(ctx context.Context, window time.Duration, nets []IPRange) (uint64, error)
	BaselineCountries(ctx context.Context, days int, minN uint64, nets []IPRange) (map[string]struct{}, error)
	RecentEdges(ctx context.Context, window time.Duration, limit int, nets []IPRange) ([]EdgeRow, error)
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
