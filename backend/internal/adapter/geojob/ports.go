package geojob

import (
	"context"

	"geoatlas/internal/model"
)

// Store — порт maintenance/backfill (реализация: *clickhouse.MaintenanceStore).
// geojob не импортирует migrate.
type Store interface {
	BackfillEdgesAgg(ctx context.Context) error
	BackfillGeoEdgesAgg(ctx context.Context) error
	EnrichLogsMissingGeo(ctx context.Context, geo GeoResolver, lookbackDays int) (int, error)
	RebuildGeoEdgesLookback(ctx context.Context, lookbackDays int) error
}

// GeoResolver — минимум для EnrichLogsMissingGeo (совпадает с geostore.GeoResolver).
type GeoResolver interface {
	RangeCount() int
	Lookup(ipStr string) model.GeoLookup
}

// GeoIndex — reload + lookup для backfill (реализация: *geostore.ReloadableGeoIndex).
type GeoIndex interface {
	Reload(ctx context.Context) error
	RangeCount() int
	Lookup(ipStr string) model.GeoLookup
}
