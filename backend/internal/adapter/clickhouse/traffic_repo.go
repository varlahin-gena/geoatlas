package clickhouse

import (
	"context"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/model"
	"network_monitor/internal/usecase/events"
)

// TrafficRepository реализует events.TrafficRepository и geo.MissingIPStore.
type TrafficRepository struct {
	apiCH ch.Conn
}

func NewTrafficRepository(apiCH ch.Conn) *TrafficRepository {
	return &TrafficRepository{apiCH: apiCH}
}

var _ events.TrafficRepository = (*TrafficRepository)(nil)

func (r *TrafficRepository) ScanRawAggsForTimeRange(ctx context.Context, tr model.TimeRange, limit int, filter string, timeout time.Duration) ([]model.RawAgg, error) {
	return query.ScanRawAggsForTimeRange(ctx, r.apiCH, tr, limit, filter, timeout)
}

func (r *TrafficRepository) ScanGeoEdgesForTimeRange(ctx context.Context, tr model.TimeRange, groupBy string, limit int, filter string, timeout time.Duration) ([]model.GeoEdgeAgg, bool, error) {
	return query.ScanGeoEdgesForTimeRange(ctx, r.apiCH, tr, groupBy, limit, filter, timeout)
}

func (r *TrafficRepository) ScanGeoMissingIPsForTimeRange(ctx context.Context, tr model.TimeRange, limit int, timeout time.Duration) ([]model.GeoMissingIPRow, error) {
	return query.ScanGeoMissingIPsForTimeRange(ctx, r.apiCH, tr, limit, timeout)
}

func (r *TrafficRepository) ScanCountrySeries(ctx context.Context, tr model.TimeRange, country string, timeout time.Duration) ([]events.SeriesPoint, int, error) {
	rows, bucket, err := query.ScanCountrySeries(ctx, r.apiCH, tr, country, timeout)
	if err != nil {
		return nil, bucket, err
	}
	out := make([]events.SeriesPoint, 0, len(rows))
	for _, p := range rows {
		out = append(out, events.SeriesPoint{
			T: p.T, Allowed: p.Allowed, Blocked: p.Blocked, Total: p.Total,
		})
	}
	return out, bucket, nil
}
