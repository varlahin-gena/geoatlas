package perrorstore

import (
	"context"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
	"network_monitor/internal/usecase/parseerrors"
)

// ParseErrorRepository реализует parseerrors.Repository.
type ParseErrorRepository struct {
	apiCH   ch.Conn
	writeCH ch.Conn
}

func NewParseErrorRepository(apiCH, writeCH ch.Conn) *ParseErrorRepository {
	return &ParseErrorRepository{apiCH: apiCH, writeCH: writeCH}
}

var _ parseerrors.Repository = (*ParseErrorRepository)(nil)

func (r *ParseErrorRepository) List(ctx context.Context, limit int, search string) ([]model.ParseErrorRow, error) {
	return ListParseErrors(ctx, r.apiCH, limit, search)
}

func (r *ParseErrorRepository) Delete(ctx context.Context, ids []string) error {
	return DeleteParseErrors(ctx, r.writeCH, ids)
}

func (r *ParseErrorRepository) DeleteAll(ctx context.Context) error {
	return DeleteAllParseErrors(ctx, r.writeCH)
}
