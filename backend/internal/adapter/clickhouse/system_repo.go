package clickhouse

import (
	"context"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
	"network_monitor/internal/storage"
	"network_monitor/internal/usecase/system"
)

// SystemRepository adapts ClickHouse-backed monitoring dependencies.
type SystemRepository struct {
	conn ch.Conn
}

func NewSystemRepository(conn ch.Conn) *SystemRepository {
	return &SystemRepository{conn: conn}
}

var (
	_ system.MetricsStore     = (*SystemRepository)(nil)
	_ system.EdgesAggReader   = (*SystemRepository)(nil)
	_ system.ClickHousePinger = (*SystemRepository)(nil)
)

func (r *SystemRepository) FetchLatest(ctx context.Context) ([]model.MetricRecord, error) {
	return storage.FetchLatestMetrics(ctx, r.conn)
}

func (r *SystemRepository) FetchHistory(ctx context.Context, keys []model.MetricKey, period, step time.Duration) (map[string][]model.HistoryPoint, error) {
	return storage.FetchMetricHistory(ctx, r.conn, keys, period, step)
}

func (r *SystemRepository) CountRows(ctx context.Context, table string) (uint64, error) {
	return storage.CountTableRows(ctx, r.conn, table)
}

func (r *SystemRepository) Status() system.EdgesAggStatus {
	status := storage.GetEdgesAggStatus()
	return system.EdgesAggStatus{
		State: status.State, Phase: status.Phase, Message: status.Message,
		RawRows: status.RawRows, AggRows: status.AggRows,
		DaysTotal: status.DaysTotal, DaysDone: status.DaysDone,
		StartedAt: status.StartedAt, UpdatedAt: status.UpdatedAt,
	}
}

func (r *SystemRepository) PreferDaily() bool {
	return storage.PreferDailyEdgesAgg()
}

func (r *SystemRepository) PreferGeo() bool {
	return storage.PreferGeoEdgesAgg()
}

func (r *SystemRepository) Ping(ctx context.Context) error {
	return r.conn.Ping(ctx)
}
