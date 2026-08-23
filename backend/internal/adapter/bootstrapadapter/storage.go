package bootstrapadapter

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/adapter/clickhouse/migrate"
	"geoatlas/internal/usecase/bootstrap"
)

// Storage — адаптер migrate.Ensure*/Backfill*/Refresh* для usecase/bootstrap.
type Storage struct {
	CH clickhouse.Conn
}

var (
	_ bootstrap.SchemaEnsurer     = (*Storage)(nil)
	_ bootstrap.AggBackfiller     = (*Storage)(nil)
	_ bootstrap.AggReadyRefresher = (*Storage)(nil)
)

func (s *Storage) EnsureHTTPSchema(ctx context.Context) error {
	return migrate.EnsureHTTPSchema(ctx, s.CH)
}

func (s *Storage) EnsureTTLOnlyDropParts(ctx context.Context) error {
	return migrate.EnsureTTLOnlyDropParts(ctx, s.CH)
}

func (s *Storage) EnsureTrafficLogsSuccess(ctx context.Context) error {
	return migrate.EnsureTrafficLogsSuccess(ctx, s.CH)
}

func (s *Storage) EnsureEdgesAggSchema(ctx context.Context) error {
	return migrate.EnsureEdgesAggSchema(ctx, s.CH)
}

func (s *Storage) EnsureHourlyEdgesAggSchema(ctx context.Context) error {
	return migrate.EnsureHourlyEdgesAggSchema(ctx, s.CH)
}

func (s *Storage) EnsureReputationRanges(ctx context.Context) error {
	return migrate.EnsureReputationRanges(ctx, s.CH)
}

func (s *Storage) BackfillEdgesAgg(ctx context.Context) error {
	return migrate.BackfillEdgesAgg(ctx, s.CH)
}

func (s *Storage) BackfillGeoEdgesAgg(ctx context.Context) error {
	return migrate.BackfillGeoEdgesAgg(ctx, s.CH)
}

func (s *Storage) BackfillHourlyEdgesAgg(ctx context.Context) error {
	return migrate.BackfillHourlyEdgesAgg(ctx, s.CH)
}

func (s *Storage) RefreshEdgesAggReady(ctx context.Context) error {
	return migrate.RefreshEdgesAggReady(ctx, s.CH)
}

func (s *Storage) RefreshGeoEdgesAggReady(ctx context.Context) error {
	return migrate.RefreshGeoEdgesAggReady(ctx, s.CH)
}

func (s *Storage) RefreshHourlyEdgesAggReady(ctx context.Context) error {
	return migrate.RefreshHourlyEdgesAggReady(ctx, s.CH)
}
