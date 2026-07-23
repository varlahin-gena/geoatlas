package geojob

import (
	"context"

	"network_monitor/internal/model"
)

// Store — порт maintenance/backfill (реализация: *clickhouse.MaintenanceStore).
// geojob не импортирует storage/migrate.
type Store interface {
	BackfillEdgesAgg(ctx context.Context) error
	BackfillGeoEdgesAgg(ctx context.Context) error
	EnrichLogsMissingGeo(ctx context.Context, geo GeoResolver, lookbackDays int) (int, error)
	RebuildGeoEdgesLookback(ctx context.Context, lookbackDays int) error
}

// GeoResolver — минимум для EnrichLogsMissingGeo (совпадает со storage.GeoResolver).
type GeoResolver interface {
	RangeCount() int
	Lookup(ipStr string) model.GeoLookup
}

// GeoIndex — reload + lookup для backfill (реализация: *clickhouse.ReloadableGeoIndex).
type GeoIndex interface {
	Reload(ctx context.Context) error
	RangeCount() int
	Lookup(ipStr string) model.GeoLookup
}
