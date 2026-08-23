package ingeststore

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/model"
)

// InsertParseErrors пакетно записывает нераспознанные строки.
func InsertParseErrors(ctx context.Context, ch clickhouse.Conn, items []model.ParseError) error {
	if len(items) == 0 {
		return nil
	}
	batch, err := ch.PrepareBatch(ctx, "INSERT INTO parse_errors (timestamp, vendor, reason, raw)")
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := batch.Append(it.Timestamp, it.Vendor, it.Reason, it.Raw); err != nil {
			_ = batch.Abort()
			return err
		}
	}
	return batch.Send()
}
