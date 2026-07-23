package clickhouse

import (
	"context"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
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

// ListParseErrors возвращает последние ошибки (с опциональным поиском по raw/reason).
func ListParseErrors(ctx context.Context, ch clickhouse.Conn, limit int, search string) ([]model.ParseErrorRow, error) {
	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString("SELECT toString(id) AS id, timestamp, vendor, reason, raw FROM parse_errors")
	if search != "" {
		sb.WriteString(" WHERE positionCaseInsensitive(raw, ?) > 0 OR positionCaseInsensitive(reason, ?) > 0")
		args = append(args, search, search)
	}
	sb.WriteString(" ORDER BY timestamp DESC LIMIT ?")
	args = append(args, uint64(limit))

	rows, err := ch.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ParseErrorRow, 0, limit)
	for rows.Next() {
		var r model.ParseErrorRow
		if err := rows.Scan(&r.ID, &r.Timestamp, &r.Vendor, &r.Reason, &r.Raw); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteParseErrors удаляет записи по списку id (lightweight delete).
func DeleteParseErrors(ctx context.Context, ch clickhouse.Conn, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := "DELETE FROM parse_errors WHERE toString(id) IN (" + strings.Join(placeholders, ",") + ")"
	return ch.Exec(ctx, q, args...)
}

// DeleteAllParseErrors очищает таблицу целиком.
func DeleteAllParseErrors(ctx context.Context, ch clickhouse.Conn) error {
	return ch.Exec(ctx, "TRUNCATE TABLE parse_errors")
}
