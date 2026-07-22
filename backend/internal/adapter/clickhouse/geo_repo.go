package clickhouse

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
	"network_monitor/internal/storage"
	"network_monitor/internal/usecase/geo"
)

// GeoRepository реализует geo.RangeStore.
type GeoRepository struct {
	apiCH   ch.Conn
	writeCH ch.Conn
}

func NewGeoRepository(apiCH, writeCH ch.Conn) *GeoRepository {
	return &GeoRepository{apiCH: apiCH, writeCH: writeCH}
}

var _ geo.RangeStore = (*GeoRepository)(nil)

func (r *GeoRepository) apiConn() ch.Conn {
	if r.apiCH != nil {
		return r.apiCH
	}
	return r.writeCH
}

func (r *GeoRepository) Replace(ctx context.Context, ranges []model.GeoRange) (int, error) {
	return storage.ReplaceGeoRanges(ctx, r.writeCH, ranges)
}

func (r *GeoRepository) Load(ctx context.Context) ([]model.GeoRange, error) {
	return storage.LoadGeoRanges(ctx, r.writeCH)
}

func (r *GeoRepository) Count(ctx context.Context) (int, error) {
	return storage.CountGeoRanges(ctx, r.apiConn())
}

func (r *GeoRepository) FindByIP(ctx context.Context, ipStr string) (model.GeoRange, bool, error) {
	return storage.FindGeoRangeByIP(ctx, r.apiConn(), ipStr)
}

func (r *GeoRepository) ListPage(ctx context.Context, limit int, q string) (items []model.GeoRange, total, filtered int, truncated bool, err error) {
	return storage.ListGeoRangesPage(ctx, r.apiConn(), limit, q)
}
