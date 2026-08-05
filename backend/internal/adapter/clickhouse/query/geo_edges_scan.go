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

// ScanGeoEdgesForTimeRange читает рёбра, свёрнутые по city|country|ip|subnet.
// city/country + days → pre-agg таблицы при готовности, иначе GROUP BY geo из traffic_logs;
// ip/subnet и minutes/hours/absolute → GROUP BY geo-колонок traffic_logs (без O(n) GeoIP в Go).
// (nil, false, nil) — нужен fallback на live IP-путь (редкий default).
func ScanGeoEdgesForTimeRange(
	ctx context.Context,
	ch clickhouse.Conn,
	tr model.TimeRange,
	groupBy string,
	limit int,
	filter string,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, bool, error) {
	switch groupBy {
	case "city", "country", "ip", "subnet":
	default:
		return nil, false, nil
	}

	tr = promoteHoursToGeoDays(tr, groupBy)

	switch tr.Mode {
	case "days":
		// У IP/subnet нет daily geo-агрегата; читаем traffic_logs.
		// Иначе top-N из traffic_edges_daily забит LAN-парами без lat → пустая карта.
		if groupBy == "ip" || groupBy == "subnet" {
			rows, err := scanGeoFromLogsRelative(ctx, ch, groupBy, "days", tr.Amount, limit, filter, timeout)
			if err != nil {
				return nil, false, err
			}
			return rows, true, nil
		}
		// city|country: pre-agg если готов — даже пустой ответ (нет данных за период).
		// Не падаем обратно на traffic_logs: иначе каждый «пустой» день = cold GROUP BY на миллионы строк.
		if aggstate.PreferGeoEdgesAgg() {
			table := sqlclause.GeoEdgesTable(groupBy)
			if table != "" {
				rows, err := scanGeoEdgesDays(ctx, ch, table, tr.Amount, limit, filter, timeout)
				if err != nil {
					slog.Warn("geo edges daily scan failed, falling back to traffic_logs",
						"group_by", groupBy, "err", err)
				} else {
					return rows, true, nil
				}
			}
		}
		rows, err := scanGeoFromLogsRelative(ctx, ch, groupBy, "days", tr.Amount, limit, filter, timeout)
		if err != nil {
			return nil, false, err
		}
		return rows, true, nil
	case "minutes", "hours":
		rows, err := scanGeoFromLogsRelative(ctx, ch, groupBy, tr.Mode, tr.Amount, limit, filter, timeout)
		if err != nil {
			return nil, false, err
		}
		return rows, true, nil
	case "absolute":
		if !tr.To.After(tr.From) {
			return nil, false, nil
		}
		rows, err := scanGeoFromLogsAbsolute(ctx, ch, groupBy, tr.From, tr.To, limit, filter, timeout)
		if err != nil {
			return nil, false, err
		}
		return rows, true, nil
	default:
		return nil, false, nil
	}
}

// promoteHoursToGeoDays: city/country + hours кратные суткам → days, если pre-agg готов.
// Иначе UI «1 день» как hours=24 всегда cold-сканит traffic_logs.
func promoteHoursToGeoDays(tr model.TimeRange, groupBy string) model.TimeRange {
	if groupBy != "city" && groupBy != "country" {
		return tr
	}
	if tr.Mode != "hours" || tr.Amount < 24 || tr.Amount%24 != 0 {
		return tr
	}
	if !aggstate.PreferGeoEdgesAgg() {
		return tr
	}
	return model.TimeRange{Mode: "days", Amount: tr.Amount / 24}
}

func scanGeoFromLogsSelect(srcKey, dstKey, srcLabel, dstLabel, whereExtra string) string {
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
		FROM traffic_logs
		WHERE %s
		GROUP BY src_key, dst_key
		ORDER BY coord_weight DESC, cnt DESC
	`, srcKey, dstKey, srcLabel, dstLabel,
		sqlclause.CountIfBlockedSQL(), sqlclause.CountIfAllowedSQL(),
		sqlclause.GeoCoordOK, sqlclause.GeoCoordOK, sqlclause.GeoCoordOK, sqlclause.GeoCoordOK,
		sqlclause.CoordWeightSQL(),
		whereExtra)
}

func scanGeoFromLogsRelative(
	ctx context.Context,
	ch clickhouse.Conn,
	groupBy, mode string,
	amount, limit int,
	filter string,
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
	where := fmt.Sprintf("timestamp >= now() - INTERVAL ? %s%s", unit, sqlclause.ActionWhereSQL(filter))
	// ClickHouse: ORDER BY → LIMIT → SETTINGS (SETTINGS must be last).
	q := scanGeoFromLogsSelect(srcKey, dstKey, srcLabel, dstLabel, where) +
		"\n\t\t" + limitClause(limit) + AggSettings()

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rows, err := ch.Query(qctx, q, amount)
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
	limit int,
	filter string,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs(groupBy)
	where := fmt.Sprintf("timestamp >= ? AND timestamp < ?%s", sqlclause.ActionWhereSQL(filter))
	q := scanGeoFromLogsSelect(srcKey, dstKey, srcLabel, dstLabel, where) +
		"\n\t\t" + limitClause(limit) + AggSettings()

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rows, err := ch.Query(qctx, q, from, to)
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
	days, limit int,
	filter string,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	if days < 1 {
		days = 1
	}
	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	having := sqlclause.HavingAggFilterSQL(filter)

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
	`, table, having, sqlclause.OrderByGeoAggFilterSQL(filter), limitClause(limit), AggSettings())

	rows, err := ch.Query(qctx, q, days)
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
