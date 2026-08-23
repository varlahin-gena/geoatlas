package geo

import (
	"context"
	"io"
	"time"

	"geoatlas/internal/geoip"
	"geoatlas/internal/model"
	"geoatlas/internal/mapagg"
)

const MaxEnterpriseNets = 200

// EnterpriseNetStore — отмеченные сети предприятия (отдельно от geo_ranges).
type EnterpriseNetStore interface {
	ListEnterpriseNets(ctx context.Context) ([]model.EnterpriseNet, error)
	UpsertEnterpriseNet(ctx context.Context, net model.EnterpriseNet) error
	DeleteEnterpriseNet(ctx context.Context, startIP, endIP uint32) error
	CountEnterpriseNets(ctx context.Context) (int, error)
}

// RangeStore — персистентность geo_ranges в ClickHouse.
type RangeStore interface {
	Replace(ctx context.Context, ranges []model.GeoRange) (int, error)
	Truncate(ctx context.Context) error
	Load(ctx context.Context) ([]model.GeoRange, error)
	Count(ctx context.Context) (int, error)
	FindByIP(ctx context.Context, ipStr string) (model.GeoRange, bool, error)
	ListPage(ctx context.Context, limit int, q string) (items []model.GeoRange, total, filtered int, truncated bool, err error)
}

// MissingIPStore — IP без координат в traffic_logs.
type MissingIPStore interface {
	ScanGeoMissingIPsForTimeRange(ctx context.Context, tr model.TimeRange, limit int, timeout time.Duration) ([]model.GeoMissingIPRow, error)
}

// GeoIndex — in-memory GeoIP.
type GeoIndex interface {
	RangeCount() int
	IndexReady() bool
	ApproxBytes() uint64
	Lookup(ipStr string) model.GeoLookup
	LookupRange(ipStr string) (model.GeoRange, bool)
	CollectRanges(limit int, q string) (items []model.GeoRange, total, filtered int, truncated bool)
	ReplaceRanges(ranges []model.GeoRange)
	ReplaceNormalizedRanges(ranges []model.GeoRange)
	ReplaceBuiltSnapshot(built *geoip.BuiltSnapshot)
}

// GeoJobScheduler — фоновый reload/backfill.
type GeoJobScheduler interface {
	ScheduleReloadAndEnrich(parent context.Context, timeout time.Duration)
}

// RangeCodec — парсинг/нормализация CSV и CIDR (инфраструктурная обёртка над geoip).
type RangeCodec interface {
	ReadCSV(r io.Reader) ([]model.GeoRange, error)
	ReadCSVSnapshot(r io.Reader) ([]model.GeoRange, *geoip.BuiltSnapshot, error)
	WriteCSV(w io.Writer, ranges []model.GeoRange) error
	Normalize(ranges []model.GeoRange) (clean []model.GeoRange, skipped int)
	CheckNonOverlapping(ranges []model.GeoRange) error
	ParseEntry(network, country, region, city string, lat, lon float64) (model.GeoRange, error)
	ParseNetwork(network string) (start, end uint32, ok bool)
	FormatNetwork(start, end uint32) string
}

// GeoLookuper — для фильтрации missing по live Lookup.
type GeoLookuper = mapagg.GeoLookuper
