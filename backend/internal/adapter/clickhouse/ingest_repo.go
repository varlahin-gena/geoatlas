package clickhouse

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
	"network_monitor/internal/storage"
	usecaseingest "network_monitor/internal/usecase/ingest"
)

// IngestRepository реализует insert-порты usecase/ingest.
type IngestRepository struct {
	writeCH ch.Conn
}

func NewIngestRepository(writeCH ch.Conn) *IngestRepository {
	return &IngestRepository{writeCH: writeCH}
}

var (
	_ usecaseingest.TrafficLogInserter = (*IngestRepository)(nil)
	_ usecaseingest.ParseErrorInserter = (*IngestRepository)(nil)
)

func (r *IngestRepository) InsertTrafficLogs(ctx context.Context, logs []model.TrafficLog) error {
	return storage.InsertTrafficLogs(ctx, r.writeCH, logs)
}

func (r *IngestRepository) InsertParseErrors(ctx context.Context, items []model.ParseError) error {
	return storage.InsertParseErrors(ctx, r.writeCH, items)
}
