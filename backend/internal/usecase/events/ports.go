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
}

// GeoLookuper — live GeoIP для fallback-пути по IP.
type GeoLookuper = mapagg.GeoLookuper

// ReputationLookuper — live репутация для groupBy=ip.
type ReputationLookuper interface {
	Lookup(ipStr string) []model.ReputationHit
}
