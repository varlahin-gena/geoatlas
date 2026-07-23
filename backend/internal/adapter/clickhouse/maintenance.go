package clickhouse

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/geojob"
	"network_monitor/internal/storage"
)

// MaintenanceStore — edges/geo backfill и enrich для geojob.Scheduler.
// Единственная точка, где geojob касается storage (не импортирует migrate напрямую).
type MaintenanceStore struct {
	ch ch.Conn
}

func NewMaintenanceStore(conn ch.Conn) *MaintenanceStore {
	return &MaintenanceStore{ch: conn}
}

var _ geojob.Store = (*MaintenanceStore)(nil)

func (m *MaintenanceStore) BackfillEdgesAgg(ctx context.Context) error {
	if m == nil || m.ch == nil {
		return nil
	}
	return storage.BackfillEdgesAgg(ctx, m.ch)
}

func (m *MaintenanceStore) BackfillGeoEdgesAgg(ctx context.Context) error {
	if m == nil || m.ch == nil {
		return nil
	}
	return storage.BackfillGeoEdgesAgg(ctx, m.ch)
}

func (m *MaintenanceStore) EnrichLogsMissingGeo(ctx context.Context, geo geojob.GeoResolver, lookbackDays int) (int, error) {
	if m == nil || m.ch == nil || geo == nil {
		return 0, nil
	}
	return storage.EnrichLogsMissingGeo(ctx, m.ch, geo, lookbackDays)
}

func (m *MaintenanceStore) RebuildGeoEdgesLookback(ctx context.Context, lookbackDays int) error {
	if m == nil || m.ch == nil {
		return nil
	}
	return storage.RebuildGeoEdgesLookback(ctx, m.ch, lookbackDays)
}
