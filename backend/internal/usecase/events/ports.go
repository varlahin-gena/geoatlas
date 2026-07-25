package events

import (
	"context"
	"time"

	"network_monitor/internal/mapagg"
	"network_monitor/internal/model"
)

// TrafficRepository — чтение агрегатов для карты.
type TrafficRepository interface {
	ScanRawAggsForTimeRange(ctx context.Context, tr model.TimeRange, limit int, filter string, timeout time.Duration) ([]model.RawAgg, error)
	ScanGeoEdgesForTimeRange(ctx context.Context, tr model.TimeRange, groupBy string, limit int, filter string, timeout time.Duration) ([]model.GeoEdgeAgg, bool, error)
	ScanCountrySeries(ctx context.Context, tr model.TimeRange, country string, timeout time.Duration) ([]SeriesPoint, int, error)
}

// SeriesPoint — точка временного ряда страны (sparkline).
type SeriesPoint struct {
	T       time.Time `json:"t"`
	Allowed uint64    `json:"allowed"`
	Blocked uint64    `json:"blocked"`
	Total   uint64    `json:"total"`
}

// GeoLookuper — live GeoIP для fallback-пути по IP.
type GeoLookuper = mapagg.GeoLookuper

// ReputationLookuper — live репутация для groupBy=ip.
type ReputationLookuper interface {
	Lookup(ipStr string) []model.ReputationHit
}
