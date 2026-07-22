package bootstrapadapter

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/storage"
	"network_monitor/internal/usecase/bootstrap"
)

// Storage — адаптер storage.Ensure*/Backfill*/Refresh* для usecase/bootstrap.
type Storage struct {
	CH clickhouse.Conn
}

var (
	_ bootstrap.SchemaEnsurer    = (*Storage)(nil)
	_ bootstrap.AggBackfiller    = (*Storage)(nil)
	_ bootstrap.AggReadyRefresher = (*Storage)(nil)
)

func (s *Storage) EnsureTTLOnlyDropParts(ctx context.Context) error {
	return storage.EnsureTTLOnlyDropParts(ctx, s.CH)
}

func (s *Storage) EnsureTrafficLogsSuccess(ctx context.Context) error {
	return storage.EnsureTrafficLogsSuccess(ctx, s.CH)
}

func (s *Storage) EnsureEdgesAggSchema(ctx context.Context) error {
	return storage.EnsureEdgesAggSchema(ctx, s.CH)
}

func (s *Storage) EnsureGeoEdgesAggSchema(ctx context.Context) error {
	return storage.EnsureGeoEdgesAggSchema(ctx, s.CH)
}

func (s *Storage) BackfillEdgesAgg(ctx context.Context) error {
	return storage.BackfillEdgesAgg(ctx, s.CH)
}

func (s *Storage) BackfillGeoEdgesAgg(ctx context.Context) error {
	return storage.BackfillGeoEdgesAgg(ctx, s.CH)
}

func (s *Storage) RefreshEdgesAggReady(ctx context.Context) error {
	return storage.RefreshEdgesAggReady(ctx, s.CH)
}

func (s *Storage) RefreshGeoEdgesAggReady(ctx context.Context) error {
	return storage.RefreshGeoEdgesAggReady(ctx, s.CH)
}
