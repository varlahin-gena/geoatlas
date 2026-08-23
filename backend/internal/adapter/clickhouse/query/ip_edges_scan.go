package query

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/adapter/clickhouse/sqlclause"
	"geoatlas/internal/model"
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

	enriched := ipEdgesEnrichOverlaySQL(inner)

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
	`, srcKey, dstKey, srcLabel, dstLabel, enriched, having, order, limitClause(sel.Limit), AggSettings())

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

// ipEdgesEnrichOverlaySQL wraps an aggregated IP-edges subquery with ga_geo_enrich_ip.
func ipEdgesEnrichOverlaySQL(inner string) string {
	return fmt.Sprintf(`
		SELECT
			e.src_ip AS src_ip,
			e.dst_ip AS dst_ip,
			e.cnt AS cnt,
			e.blocked_cnt AS blocked_cnt,
			e.allowed_cnt AS allowed_cnt,
			e.bytes_sent AS bytes_sent,
			e.bytes_recv AS bytes_recv,
			e.packets_sent AS packets_sent,
			e.packets_recv AS packets_recv,
			if(e.src_lat_sum = 0 AND e.src_lon_sum = 0 AND (sg.lat != 0 OR sg.lon != 0),
				sg.lat * toFloat64(greatest(e.coord_weight, toUInt64(1))), e.src_lat_sum) AS src_lat_sum,
			if(e.src_lat_sum = 0 AND e.src_lon_sum = 0 AND (sg.lat != 0 OR sg.lon != 0),
				sg.lon * toFloat64(greatest(e.coord_weight, toUInt64(1))), e.src_lon_sum) AS src_lon_sum,
			if(e.dst_lat_sum = 0 AND e.dst_lon_sum = 0 AND (dg.lat != 0 OR dg.lon != 0),
				dg.lat * toFloat64(greatest(e.coord_weight, toUInt64(1))), e.dst_lat_sum) AS dst_lat_sum,
			if(e.dst_lat_sum = 0 AND e.dst_lon_sum = 0 AND (dg.lat != 0 OR dg.lon != 0),
				dg.lon * toFloat64(greatest(e.coord_weight, toUInt64(1))), e.dst_lon_sum) AS dst_lon_sum,
			if(e.coord_weight = 0
				AND (sg.lat != 0 OR sg.lon != 0)
				AND (dg.lat != 0 OR dg.lon != 0),
				toUInt64(1), e.coord_weight) AS coord_weight,
			e.last_action AS last_action,
			e.rule AS rule,
			e.proto AS proto,
			e.src_port AS src_port,
			e.dst_port AS dst_port,
			e.device AS device,
			e.src_zone AS src_zone,
			e.dst_zone AS dst_zone,
			if(%[2]s AND sg.country != '', sg.country, e.src_country) AS src_country,
			if(%[3]s AND dg.country != '', dg.country, e.dst_country) AS dst_country,
			if(%[4]s AND sg.city != '', sg.city, e.src_city) AS src_city,
			if(%[5]s AND dg.city != '', dg.city, e.dst_city) AS dst_city
		FROM (%[1]s) AS e
		LEFT JOIN %[6]s AS sg ON e.src_ip = sg.ip
		LEFT JOIN %[6]s AS dg ON e.dst_ip = dg.ip
	`, inner,
		sqlclause.CountryNeedsSQL("e.src_country"),
		sqlclause.CountryNeedsSQL("e.dst_country"),
		sqlclause.CityNeedsSQL("e.src_city"),
		sqlclause.CityNeedsSQL("e.dst_city"),
		sqlclause.GeoEnrichIPTable,
	)
}
