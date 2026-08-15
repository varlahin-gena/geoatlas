package trafficstore

import (
	"context"
	"log/slog"
	"strings"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/aggstate"
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

func scopeCtx(ctx context.Context, dataSource string) context.Context {
	ctx = aggstate.WithAgg(ctx, aggstate.Default)
	if strings.EqualFold(strings.TrimSpace(dataSource), "backup") {
		return query.WithTables(ctx, query.BackupTables())
	}
	return ctx
}

// composeMapAgg: geo-путь (даже пустой) не должен падать во второй raw scan.
func composeMapAgg(groupBy string, geoRows []model.GeoEdgeAgg, geoOK bool, raws []model.RawAgg) events.MapAggScanResult {
	if geoOK {
		return events.MapAggScanResult{Source: "geo_" + groupBy, GeoEdges: geoRows}
	}
	return events.MapAggScanResult{Source: "ip_live_" + groupBy, Raws: raws}
}

func (r *TrafficRepository) ScanMapAggs(ctx context.Context, tr model.TimeRange, q events.MapScanQuery, timeout time.Duration) (events.MapAggScanResult, error) {
	ctx = scopeCtx(ctx, q.DataSource)
	sel := query.MapSelect{Limit: q.Limit, Filter: q.Filter, Country: q.Country, Query: q.Query}
	geoRows, ok, err := query.ScanGeoEdgesForTimeRange(ctx, r.apiCH, tr, q.GroupBy, sel, timeout)
	if err != nil {
		// Не роняем /api/events: pre-agg/лог-geo могут быть ещё не готовы при старте.
		slog.Warn("ScanMapAggs: geo edges scan failed, falling back to raw IP pairs",
			"group_by", q.GroupBy, "err", err)
		ok = false
	}
	if ok {
		return composeMapAgg(q.GroupBy, geoRows, true, nil), nil
	}

	raws, err := query.ScanRawAggsForTimeRange(ctx, r.apiCH, tr, sel, timeout)
	if err != nil {
		return events.MapAggScanResult{}, err
	}
	return composeMapAgg(q.GroupBy, nil, false, raws), nil
}

func (r *TrafficRepository) ScanGeoMissingIPsForTimeRange(ctx context.Context, tr model.TimeRange, limit int, timeout time.Duration) ([]model.GeoMissingIPRow, error) {
	return query.ScanGeoMissingIPsForTimeRange(ctx, r.apiCH, tr, limit, timeout)
}

func (r *TrafficRepository) ScanCountrySeries(ctx context.Context, tr model.TimeRange, country, dataSource string, timeout time.Duration) ([]events.SeriesPoint, int, error) {
	ctx = scopeCtx(ctx, dataSource)
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
