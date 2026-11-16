package anomaly

import (
	"context"
	"time"

	"geoatlas/internal/model"
)

// EventStore — журнал аномалий.
type EventStore interface {
	Insert(ctx context.Context, events []Event) error
	List(ctx context.Context, q ListQuery) ([]Event, error)
	ExistingFingerprints(ctx context.Context, fps []string) (map[string]struct{}, error)
	ActiveSuppressions(ctx context.Context, keys []SuppressionKey, now time.Time) (map[SuppressionKey]struct{}, error)
	RecentSuppressionKeys(ctx context.Context, code string, keys []SuppressionKey, since time.Time) (map[SuppressionKey]struct{}, error)
	Ack(ctx context.Context, fingerprint, by string, suppressFor time.Duration) error
	// OpenBySuppression — есть незакрытое событие с этим ключом.
	OpenBySuppression(ctx context.Context, key SuppressionKey) (bool, error)
	Assign(ctx context.Context, fingerprint, assignedTo, by string) error
	AssignIfEmpty(ctx context.Context, fingerprint, assignedTo, by string) error
	CountSummary(ctx context.Context, since time.Time) (Summary, error)
}

// EnterpriseNetSource — отмеченные подсети предприятия.
type EnterpriseNetSource interface {
	ListEnterpriseNets(ctx context.Context) ([]model.EnterpriseNet, error)
}

// TrafficScanner — SQL-сканы для детекторов (без CH-типов в usecase).
type TrafficScanner interface {
	OldestLogTime(ctx context.Context) (time.Time, error)
	// PortScan — уникальные порты назначения среди блокировок и отказов. Частные источники входят всегда.
	PortScan(ctx context.Context, window time.Duration, portsTh, eventsTh int, nets []IPRange) ([]PortScanHit, error)
	BlockedCount(ctx context.Context, start, end time.Time, net *IPRange) (uint64, error)
	// BlockedCounts — current/previous для каждой сети одним проходом.
	// len(результата) == len(nets), порядок совпадает. Пустой nets → nil, nil.
	BlockedCounts(ctx context.Context, currStart, currEnd, prevStart, prevEnd time.Time, nets []IPRange) ([]BlockedWindowCount, error)
	// BlockedPeers — топ источников и назначений блокировок одной сети.
	BlockedPeers(ctx context.Context, start, end time.Time, net IPRange, limit int) (src, dst []BlockedPeer, err error)
	// HostCountries — пары «источник предприятия → страна» в текущем окне.
	HostCountries(ctx context.Context, window time.Duration, minN uint64, nets []IPRange) ([]HostCountryHit, error)
	// HostCountryBaseline — пары, которые уже встречались до текущего окна. Ключ: ip|country.
	HostCountryBaseline(ctx context.Context, days int, minN uint64, nets []IPRange) (map[string]struct{}, error)
	// ReputationEdges — рёбра окна, у которых src или dst попал в reputation_ranges.
	// Отбор по пересечению со списком, не по топу объёма.
	ReputationEdges(ctx context.Context, window time.Duration, minEvents uint64, limit int, nets []IPRange) ([]EdgeRow, error)
	KnownPairs(ctx context.Context, pairs [][2]string, lookback time.Duration) (map[string]struct{}, error)
	ByteSurge(ctx context.Context, window time.Duration, floor, absMin uint64, nets []IPRange) ([]ByteSurgeHit, error)
	Beaconing(ctx context.Context, lookback time.Duration, minHours int, maxAvgBytes uint64, nets []IPRange) ([]BeaconingHit, error)
	// SeenIPs — какие из ips встречались в [from, to).
	SeenIPs(ctx context.Context, ips []string, from, to time.Time) (map[string]struct{}, error)
	// LateralFanout — много назначений внутри сетей предприятия. exclude — источники, которые не алертят.
	LateralFanout(ctx context.Context, window time.Duration, hostsTh, eventsTh int, nets []IPRange, exclude []string) ([]LateralFanoutHit, error)
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
