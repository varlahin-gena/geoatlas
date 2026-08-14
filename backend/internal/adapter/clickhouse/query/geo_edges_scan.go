package query

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/aggstate"
	"network_monitor/internal/adapter/clickhouse/sqlclause"
	"network_monitor/internal/model"
)

// MapSelect — LIMIT + action-фильтр + опциональный country/q (bind-args).
type MapSelect struct {
	Limit   int
	Filter  string
	Country string
	Query   string
}

func (s MapSelect) scope() sqlclause.MapScope {
	return sqlclause.MapScope{Country: s.Country, Query: s.Query}
}

// ScanGeoEdgesForTimeRange читает рёбра, свёрнутые по city|country|ip|subnet.
// city/country + days → pre-agg таблицы при готовности, иначе GROUP BY geo из traffic_logs;
// ip/subnet и minutes/hours/absolute → GROUP BY geo-колонок traffic_logs (без O(n) GeoIP в Go).
// (nil, false, nil) — нужен fallback на live IP-путь (редкий default).
func ScanGeoEdgesForTimeRange(
	ctx context.Context,
	ch clickhouse.Conn,
	tr model.TimeRange,
	groupBy string,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, bool, error) {
	switch groupBy {
	case "city", "country", "ip", "subnet":
	default:
		return nil, false, nil
	}

	tables := TablesOf(ctx)
	switch groupBy {
	case "ip", "subnet":
		tr = promoteHoursToDays(tr, tables.IsBackup() || aggstate.PreferDailyEdgesAgg())
		tr = promoteMinutesToHours(tr, tables.IsBackup() || aggstate.PreferHourlyEdgesAgg())
	case "city", "country":
		tr = promoteHoursToDays(tr, tables.IsBackup() || aggstate.PreferGeoEdgesAgg())
		tr = promoteMinutesToHours(tr, tables.IsBackup() || aggstate.PreferHourlyEdgesAgg())
	}

	switch tr.Mode {
	case "days":
		if groupBy == "ip" || groupBy == "subnet" {
			tryDaily := tables.IsBackup() || aggstate.PreferDailyEdgesAgg()
			if tryDaily && tables.EdgesDaily != "" {
				rows, err := scanIPEdgesRelative(ctx, ch, tables.EdgesDaily, "day", "DAY", tr.Amount, groupBy, sel, timeout)
				if err != nil {
					slog.Warn("ip edges daily scan failed, falling back to traffic_logs",
						"group_by", groupBy, "err", err)
				} else {
					return rows, true, nil
				}
			}
			rows, err := scanGeoFromLogsRelative(ctx, ch, groupBy, "days", tr.Amount, sel, timeout)
			if err != nil {
				return nil, false, err
			}
			return rows, true, nil
		}
		// city|country: pre-agg если готов — даже пустой ответ (нет данных за период).
		tryEdges := tables.IsBackup() || aggstate.PreferGeoEdgesAgg()
		if tryEdges {
			table := tables.GeoEdges(groupBy)
			if table != "" {
				rows, err := scanGeoEdgesDays(ctx, ch, table, tr.Amount, sel, timeout)
				if err != nil {
					slog.Warn("geo edges daily scan failed, falling back to traffic_logs",
						"group_by", groupBy, "table", table, "err", err)
				} else {
					return rows, true, nil
				}
			}
		}
		rows, err := scanGeoFromLogsRelative(ctx, ch, groupBy, "days", tr.Amount, sel, timeout)
		if err != nil {
			return nil, false, err
		}
		return rows, true, nil
	case "minutes", "hours":
		tryHourly := (tables.IsBackup() || aggstate.PreferHourlyEdgesAgg()) && tr.Mode == "hours"
		if tryHourly && tables.EdgesHourly != "" {
			rows, err := scanIPEdgesRelative(ctx, ch, tables.EdgesHourly, "hour", "HOUR", tr.Amount, groupBy, sel, timeout)
			if err != nil {
				slog.Warn("ip edges hourly scan failed, falling back to traffic_logs",
					"group_by", groupBy, "err", err)
			} else {
				return rows, true, nil
			}
		}
		rows, err := scanGeoFromLogsRelative(ctx, ch, groupBy, tr.Mode, tr.Amount, sel, timeout)
		if err != nil {
			return nil, false, err
		}
		return rows, true, nil
	case "absolute":
		if !tr.To.After(tr.From) {
			return nil, false, nil
		}
		tryHourly := tables.IsBackup() || aggstate.PreferHourlyEdgesAgg()
		if tryHourly && tables.EdgesHourly != "" && tr.To.Sub(tr.From) <= 7*24*time.Hour {
			rows, err := scanIPEdgesAbsolute(ctx, ch, tables.EdgesHourly, "hour", tr.From, tr.To, groupBy, sel, timeout)
			if err != nil {
				slog.Warn("ip edges hourly absolute scan failed, falling back to traffic_logs",
					"group_by", groupBy, "err", err)
			} else {
				return rows, true, nil
			}
		}
		rows, err := scanGeoFromLogsAbsolute(ctx, ch, groupBy, tr.From, tr.To, sel, timeout)
		if err != nil {
			return nil, false, err
		}
		return rows, true, nil
	default:
		return nil, false, nil
	}
}

func promoteHoursToDays(tr model.TimeRange, prefer bool) model.TimeRange {
	if tr.Mode != "hours" || tr.Amount < 24 || tr.Amount%24 != 0 || !prefer {
		return tr
	}
	return model.TimeRange{Mode: "days", Amount: tr.Amount / 24}
}

func promoteMinutesToHours(tr model.TimeRange, prefer bool) model.TimeRange {
	if tr.Mode != "minutes" || tr.Amount < 60 || tr.Amount%60 != 0 || !prefer {
		return tr
	}
	return model.TimeRange{Mode: "hours", Amount: tr.Amount / 60}
}

// promoteHoursToGeoDays: hours кратные суткам → days, если preferEdges.
func promoteHoursToGeoDays(tr model.TimeRange, groupBy string, preferEdges bool) model.TimeRange {
	switch groupBy {
	case "city", "country", "ip", "subnet":
		return promoteHoursToDays(tr, preferEdges)
	default:
		return tr
	}
}

func scanGeoFromLogsSelect(logsTable, srcKey, dstKey, srcLabel, dstLabel, whereExtra string) string {
	// Не использовать any(src_city) AS src_city: ClickHouse подставит алиас
	// внутрь any(cityLabelExpr), получится вложенный aggregate (code 184).
	return fmt.Sprintf(`
		SELECT
			%s AS src_key,
			%s AS dst_key,
			any(%s) AS src_label,
			any(%s) AS dst_label,
			count() AS cnt,
			%s AS blocked_cnt,
			%s AS allowed_cnt,
			sum(bytes_sent) AS bytes_sent,
			sum(bytes_recv) AS bytes_recv,
			sum(packets_sent) AS packets_sent,
			sum(packets_recv) AS packets_recv,
			sumIf(src_lat, %s) AS src_lat_sum,
			sumIf(src_lon, %s) AS src_lon_sum,
			sumIf(dst_lat, %s) AS dst_lat_sum,
			sumIf(dst_lon, %s) AS dst_lon_sum,
			%s AS coord_weight,
			argMax(action, timestamp) AS last_action,
			any(rule) AS rule,
			any(proto) AS proto,
			any(src_port) AS src_port,
			any(dst_port) AS dst_port,
			any(device) AS device,
			any(src_zone) AS src_zone,
			any(dst_zone) AS dst_zone,
			any(src_country) AS out_src_country,
			any(dst_country) AS out_dst_country,
			any(src_city) AS out_src_city,
			any(dst_city) AS out_dst_city
		FROM %s
		WHERE %s
		GROUP BY src_key, dst_key
		ORDER BY coord_weight DESC, cnt DESC
	`, srcKey, dstKey, srcLabel, dstLabel,
		sqlclause.CountIfBlockedSQL(), sqlclause.CountIfAllowedSQL(),
		sqlclause.GeoCoordOK, sqlclause.GeoCoordOK, sqlclause.GeoCoordOK, sqlclause.GeoCoordOK,
		sqlclause.CoordWeightSQL(),
		logsTable, whereExtra)
}

func scanGeoFromLogsRelative(
	ctx context.Context,
	ch clickhouse.Conn,
	groupBy, mode string,
	amount int,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	var unit string
	switch mode {
	case "minutes":
		unit = "MINUTE"
	case "hours":
		unit = "HOUR"
	case "days":
		unit = "DAY"
	default:
		return nil, fmt.Errorf("unsupported geo relative mode %q", mode)
	}
	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs(groupBy)
	scopeSQL, scopeArgs := sel.scope().LogsWhere()
	where := fmt.Sprintf("timestamp >= now() - INTERVAL ? %s%s%s", unit, sqlclause.ActionWhereSQL(sel.Filter), scopeSQL)
	// ClickHouse: ORDER BY → LIMIT → SETTINGS (SETTINGS must be last).
	q := scanGeoFromLogsSelect(TablesOf(ctx).Logs, srcKey, dstKey, srcLabel, dstLabel, where) +
		"\n\t\t" + limitClause(sel.Limit) + AggSettings()

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := append([]any{amount}, scopeArgs...)
	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGeoEdgeRows(rows)
}

func scanGeoFromLogsAbsolute(
	ctx context.Context,
	ch clickhouse.Conn,
	groupBy string,
	from, to time.Time,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs(groupBy)
	scopeSQL, scopeArgs := sel.scope().LogsWhere()
	where := fmt.Sprintf("timestamp >= ? AND timestamp < ?%s%s", sqlclause.ActionWhereSQL(sel.Filter), scopeSQL)
	q := scanGeoFromLogsSelect(TablesOf(ctx).Logs, srcKey, dstKey, srcLabel, dstLabel, where) +
		"\n\t\t" + limitClause(sel.Limit) + AggSettings()

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := append([]any{from, to}, scopeArgs...)
	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGeoEdgeRows(rows)
}

func scanGeoEdgesDays(
	ctx context.Context,
	ch clickhouse.Conn,
	table string,
	days int,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	if days < 1 {
		days = 1
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	scopeHaving, scopeArgs := sel.scope().GeoAggHavingExpr()
	having := sqlclause.JoinHaving(sqlclause.HavingAggFilterSQL(sel.Filter), scopeHaving)

	q := fmt.Sprintf(`
		SELECT
			src_key, dst_key,
			anyMerge(src_label) AS src_label,
			anyMerge(dst_label) AS dst_label,
			sum(cnt) AS cnt,
			sum(blocked_cnt) AS blocked_cnt,
			sum(allowed_cnt) AS allowed_cnt,
			sum(bytes_sent) AS bytes_sent,
			sum(bytes_recv) AS bytes_recv,
			sum(packets_sent) AS packets_sent,
			sum(packets_recv) AS packets_recv,
			sum(src_lat_sum) AS src_lat_sum,
			sum(src_lon_sum) AS src_lon_sum,
			sum(dst_lat_sum) AS dst_lat_sum,
			sum(dst_lon_sum) AS dst_lon_sum,
			sum(coord_weight) AS coord_weight,
			argMaxMerge(last_action) AS last_action,
			anyMerge(rule) AS rule,
			anyMerge(proto) AS proto,
			anyMerge(src_port) AS src_port,
			anyMerge(dst_port) AS dst_port,
			anyMerge(device) AS device,
			anyMerge(src_zone) AS src_zone,
			anyMerge(dst_zone) AS dst_zone,
			anyMerge(src_country) AS src_country,
			anyMerge(dst_country) AS dst_country,
			anyMerge(src_city) AS src_city,
			anyMerge(dst_city) AS dst_city
		FROM %s
		WHERE day >= today() - ?
		GROUP BY src_key, dst_key
		%s
		%s
		%s
		%s
	`, table, having, sqlclause.OrderByGeoAggFilterSQL(sel.Filter), limitClause(sel.Limit), AggSettings())

	args := append([]any{days}, scopeArgs...)
	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGeoEdgeRows(rows)
}

func scanGeoEdgeRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.GeoEdgeAgg, error) {
	var out []model.GeoEdgeAgg
	for rows.Next() {
		var (
			r                                          model.GeoEdgeAgg
			srcLatSum, srcLonSum, dstLatSum, dstLonSum float64
			coordWeight                                uint64
		)
		if err := rows.Scan(
			&r.SrcKey, &r.DstKey,
			&r.SrcLabel, &r.DstLabel,
			&r.Count, &r.BlockedCnt, &r.AllowedCnt,
			&r.BytesSent, &r.BytesRecv, &r.PacketsSent, &r.PacketsRecv,
			&srcLatSum, &srcLonSum, &dstLatSum, &dstLonSum, &coordWeight,
			&r.LastAction, &r.Rule, &r.Proto, &r.SrcPort, &r.DstPort,
			&r.Device, &r.SrcZone, &r.DstZone,
			&r.SrcCountry, &r.DstCountry, &r.SrcCity, &r.DstCity,
		); err != nil {
			slog.Warn("geo edge row scan failed", "err", err)
			continue
		}
		if coordWeight > 0 {
			r.SrcLat = srcLatSum / float64(coordWeight)
			r.SrcLon = srcLonSum / float64(coordWeight)
			r.DstLat = dstLatSum / float64(coordWeight)
			r.DstLon = dstLonSum / float64(coordWeight)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
