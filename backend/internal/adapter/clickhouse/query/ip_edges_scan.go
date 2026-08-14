package query

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/sqlclause"
	"network_monitor/internal/model"
)

func scanIPEdgesRelative(
	ctx context.Context,
	ch clickhouse.Conn,
	table, timeCol, unit string,
	amount int,
	groupBy string,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	if amount < 1 {
		amount = 1
	}
	where := fmt.Sprintf("%s >= now() - INTERVAL ? %s", timeCol, unit)
	if timeCol == "day" {
		where = fmt.Sprintf("%s >= today() - ?", timeCol)
	}
	return scanIPEdges(ctx, ch, table, where, []any{amount}, groupBy, sel, timeout)
}

func scanIPEdgesAbsolute(
	ctx context.Context,
	ch clickhouse.Conn,
	table, timeCol string,
	from, to time.Time,
	groupBy string,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	where := fmt.Sprintf("%s >= ? AND %s < ?", timeCol, timeCol)
	return scanIPEdges(ctx, ch, table, where, []any{from, to}, groupBy, sel, timeout)
}

func scanIPEdges(
	ctx context.Context,
	ch clickhouse.Conn,
	table, where string,
	baseArgs []any,
	groupBy string,
	sel MapSelect,
	timeout time.Duration,
) ([]model.GeoEdgeAgg, error) {
	inner := fmt.Sprintf(`
		SELECT
			src_ip, dst_ip,
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
		WHERE %s
		GROUP BY src_ip, dst_ip
	`, table, where)

	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs(groupBy)
	scopeHaving, scopeArgs := sel.scope().GeoAggHavingExpr()
	having := sqlclause.JoinHaving(sqlclause.HavingAggFilterSQL(sel.Filter), scopeHaving)
	order := sqlclause.OrderByGeoAggFilterSQL(sel.Filter)

	q := fmt.Sprintf(`
		SELECT
			%s AS src_key,
			%s AS dst_key,
			any(%s) AS src_label,
			any(%s) AS dst_label,
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
			any(last_action) AS last_action,
			any(rule) AS rule,
			any(proto) AS proto,
			any(src_port) AS src_port,
			any(dst_port) AS dst_port,
			any(device) AS device,
			any(src_zone) AS src_zone,
			any(dst_zone) AS dst_zone,
			any(src_country) AS src_country,
			any(dst_country) AS dst_country,
			any(src_city) AS src_city,
			any(dst_city) AS dst_city
		FROM (%s) AS e
		GROUP BY src_key, dst_key
		%s
		%s
		%s
		%s
	`, srcKey, dstKey, srcLabel, dstLabel, inner, having, order, limitClause(sel.Limit), AggSettings())

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := append(append([]any{}, baseArgs...), scopeArgs...)
	rows, err := ch.Query(qctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGeoEdgeRows(rows)
}
