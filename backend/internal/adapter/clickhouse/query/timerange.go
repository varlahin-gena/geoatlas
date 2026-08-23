package query

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/adapter/clickhouse/sqlclause"
	"geoatlas/internal/model"
)

// ScanRawAggsForTimeRange читает пары src/dst за выбранный период.
// Относительные окна (минуты/часы/дни) фильтруются через now() ClickHouse,
// чтобы совпадать с часовым поясом сервера БД и существующими запросами.
func ScanRawAggsForTimeRange(
	ctx context.Context,
	ch clickhouse.Conn,
	tr model.TimeRange,
	sel MapSelect,
	timeout time.Duration,
) ([]model.RawAgg, error) {
	switch tr.Mode {
	case "minutes", "hours":
		return scanRawLogsRelative(ctx, ch, tr.Mode, tr.Amount, sel, timeout)
	case "days":
		tables := TablesOf(ctx)
		if tables.IsBackup() || aggstate.AggFromContext(ctx).PreferDailyEdgesAgg() {
			rows, err := scanEdgesDailyAsRaw(ctx, ch, tables.EdgesDaily, tr.Amount, sel, timeout)
			if err != nil {
				slog.Warn("edges daily scan failed, falling back to traffic_logs", "err", err)
			} else {
				return rows, nil
			}
		}
		return scanRawLogsRelative(ctx, ch, "days", tr.Amount, sel, timeout)
	case "absolute":
		if !tr.To.After(tr.From) {
			return nil, nil
		}
		return scanRawLogsAbsolute(ctx, ch, tr.From, tr.To, sel, timeout)
	default:
		return scanRawLogsRelative(ctx, ch, "days", 1, sel, timeout)
	}
}

func rawAggSelectSQL(logsTable, whereExtra, filter string, limit int) string {
	return fmt.Sprintf(`
		SELECT
			toString(src_ip) AS src_ip,
			toString(dst_ip) AS dst_ip,
			count() AS cnt,
			%s AS blocked_cnt,
			%s AS allowed_cnt,
			argMax(action, timestamp) AS last_action,
			any(rule) AS rule,
			any(proto) AS proto,
			any(src_port) AS src_port,
			any(dst_port) AS dst_port,
			any(device) AS device,
			any(src_zone) AS src_zone,
			any(dst_zone) AS dst_zone,
			any(src_country) AS src_country,
			any(dst_country) AS dst_country,
			anyIf(src_city, src_city != '') AS out_src_city,
			anyIf(dst_city, dst_city != '') AS out_dst_city,
			-- AS src_lat/src_lon нельзя: в CH 25 analyzer резолвит соседние алиасы → nested aggregate (code 184).
			anyIf(src_lat, (src_lat != 0) OR (src_lon != 0)) AS out_src_lat,
			anyIf(src_lon, (src_lat != 0) OR (src_lon != 0)) AS out_src_lon,
			anyIf(dst_lat, (dst_lat != 0) OR (dst_lon != 0)) AS out_dst_lat,
			anyIf(dst_lon, (dst_lat != 0) OR (dst_lon != 0)) AS out_dst_lon,
			sum(bytes_sent) AS bytes_sent,
			sum(bytes_recv) AS bytes_recv,
			sum(packets_sent) AS packets_sent,
			sum(packets_recv) AS packets_recv
		FROM %s
		WHERE %s
		GROUP BY src_ip, dst_ip
		%s
		%s
		%s
	`, sqlclause.CountIfBlockedSQL(), sqlclause.CountIfAllowedSQL(), logsTable, whereExtra,
		sqlclause.OrderByMapAggFilterSQL(filter), limitClause(limit), AggSettings())
}

func scanRawLogsRelative(
	ctx context.Context,
	ch clickhouse.Conn,
	mode string,
	amount int,
	sel MapSelect,
	timeout time.Duration,
) ([]model.RawAgg, error) {
	var unit string
	switch mode {
	case "minutes":
		unit = "MINUTE"
	case "hours":
		unit = "HOUR"
	case "days":
		unit = "DAY"
	default:
		return nil, fmt.Errorf("unknown relative mode %q", mode)
	}

	scopeSQL, scopeArgs := sel.scope().LogsWhere()
	where := fmt.Sprintf("timestamp >= now() - INTERVAL ? %s%s%s", unit, sqlclause.ActionWhereSQL(sel.Filter), scopeSQL)
	q := rawAggSelectSQL(TablesOf(ctx).Logs, where, sel.Filter, sel.Limit)

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]any{amount}, scopeArgs...)
	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"query raw aggregates (mode=%s amount=%d filter=%q): %w\nSQL:\n%s",
			mode, amount, sel.Filter, err, q,
		)
	}
	defer rows.Close()

	return scanRawAggRows(rows)
}

func scanRawLogsAbsolute(
	ctx context.Context,
	ch clickhouse.Conn,
	from, to time.Time,
	sel MapSelect,
	timeout time.Duration,
) ([]model.RawAgg, error) {
	scopeSQL, scopeArgs := sel.scope().LogsWhere()
	where := fmt.Sprintf("timestamp >= ? AND timestamp < ?%s%s", sqlclause.ActionWhereSQL(sel.Filter), scopeSQL)
	q := rawAggSelectSQL(TablesOf(ctx).Logs, where, sel.Filter, sel.Limit)

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]any{from, to}, scopeArgs...)
	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf(
			"query raw aggregates (from=%s to=%s filter=%q): %w\nSQL:\n%s",
			from.UTC().Format(time.RFC3339Nano), to.UTC().Format(time.RFC3339Nano), sel.Filter, err, q,
		)
	}
	defer rows.Close()

	return scanRawAggRows(rows)
}
