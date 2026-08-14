package events

import (
	"context"
	"time"

	"network_monitor/internal/mapagg"
	"network_monitor/internal/model"
)

// TrafficRepository — чтение агрегатов для карты.
type TrafficRepository interface {
	ScanMapAggs(ctx context.Context, tr model.TimeRange, q MapScanQuery, timeout time.Duration) (MapAggScanResult, error)
	ScanCountrySeries(ctx context.Context, tr model.TimeRange, country string, timeout time.Duration) ([]SeriesPoint, int, error)
}

// MapAggScanResult — готовый результат выбора источника данных для карты.
// GeoEdges используется для pre-agg/log-geo пути; Raws — для live GeoIP fallback.
type MapAggScanResult struct {
	Source   string
	GeoEdges []model.GeoEdgeAgg
	Raws     []model.RawAgg
}

// SeriesPoint — точка временного ряда страны (sparkline).
type SeriesPoint struct {
	T       time.Time `json:"t"`
	Allowed uint64    `json:"allowed"`
	Blocked uint64    `json:"blocked"`
	Total   uint64    `json:"total"`
}

// GeoLookuper — live GeoIP для raw fallback-пути по IP.
type GeoLookuper = mapagg.GeoLookuper

// ReputationLookuper — live репутация для groupBy=ip.
type ReputationLookuper interface {
	Lookup(ipStr string) []model.ReputationHit
}
