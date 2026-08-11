package query

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/sqlclause"
	"network_monitor/internal/model"
)

// SeriesPoint — bucket временного ряда для sparkline страны.
type SeriesPoint struct {
	T       time.Time
	Allowed uint64
	Blocked uint64
	Total   uint64
}

func pickSeriesBucketSeconds(tr model.TimeRange) int {
	var dur time.Duration
	switch tr.Mode {
	case "minutes":
		dur = time.Duration(tr.Amount) * time.Minute
	case "hours":
		dur = time.Duration(tr.Amount) * time.Hour
	case "days":
		dur = time.Duration(tr.Amount) * 24 * time.Hour
	case "absolute":
		if tr.To.After(tr.From) {
			dur = tr.To.Sub(tr.From)
		}
	}
	if dur <= 0 {
		dur = 24 * time.Hour
	}
	switch {
	case dur <= 2*time.Hour:
		return 5 * 60
	case dur <= 12*time.Hour:
		return 15 * 60
	case dur <= 3*24*time.Hour:
		return 60 * 60
	case dur <= 14*24*time.Hour:
		return 6 * 60 * 60
	default:
		return 24 * 60 * 60
	}
}

// ScanCountrySeries агрегирует allowed/blocked по интервалам для src_country или dst_country.
func ScanCountrySeries(
	ctx context.Context,
	ch clickhouse.Conn,
	tr model.TimeRange,
	country string,
	timeout time.Duration,
) ([]SeriesPoint, int, error) {
	country = strings.TrimSpace(country)
	if country == "" {
		return nil, 0, fmt.Errorf("country is required")
	}
	bucketSec := pickSeriesBucketSeconds(tr)

	var (
		where string
		args  []any
	)
	switch tr.Mode {
	case "minutes":
		where = "timestamp >= now() - INTERVAL ? MINUTE AND (src_country = ? OR dst_country = ?)"
		args = []any{tr.Amount, country, country}
	case "hours":
		where = "timestamp >= now() - INTERVAL ? HOUR AND (src_country = ? OR dst_country = ?)"
		args = []any{tr.Amount, country, country}
	case "days":
		where = "timestamp >= now() - INTERVAL ? DAY AND (src_country = ? OR dst_country = ?)"
		args = []any{tr.Amount, country, country}
	case "absolute":
		if !tr.To.After(tr.From) {
			return nil, bucketSec, nil
		}
		where = "timestamp >= ? AND timestamp < ? AND (src_country = ? OR dst_country = ?)"
		args = []any{tr.From, tr.To, country, country}
	default:
		where = "timestamp >= now() - INTERVAL ? DAY AND (src_country = ? OR dst_country = ?)"
		args = []any{1, country, country}
	}

	q := fmt.Sprintf(`
		SELECT
			toStartOfInterval(timestamp, INTERVAL %d SECOND) AS bucket,
			%s AS blocked_cnt,
			%s AS allowed_cnt,
			count() AS total
		FROM %s
		WHERE %s
		GROUP BY bucket
		ORDER BY bucket
		%s
	`, bucketSec, sqlclause.CountIfBlockedSQL(), sqlclause.CountIfAllowedSQL(), TablesOf(ctx).Logs, where, AggSettings())

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, bucketSec, fmt.Errorf("country series: %w\nSQL:\n%s", err, q)
	}
	defer rows.Close()

	var out []SeriesPoint
	for rows.Next() {
		var p SeriesPoint
		if err := rows.Scan(&p.T, &p.Blocked, &p.Allowed, &p.Total); err != nil {
			slog.Warn("ScanCountrySeries: row scan failed", "err", err)
			continue
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return out, bucketSec, err
	}
	return out, bucketSec, nil
}
