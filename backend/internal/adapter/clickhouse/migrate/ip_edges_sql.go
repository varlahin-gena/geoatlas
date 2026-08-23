package migrate

import (
	"fmt"

	"geoatlas/internal/adapter/clickhouse/sqlclause"
)

const (
	ipEdgesDailyTTL  = "day + INTERVAL 30 DAY DELETE"
	ipEdgesHourlyTTL = "toDateTime(hour) + INTERVAL 7 DAY DELETE"
)

func ipEdgesCreateTableSQL(name, timeCol, timeType, partitionBy, ttlExpr string) string {
	return fmt.Sprintf(`
		CREATE TABLE %s
		(
			%[2]s         %[3]s,
			src_ip        IPv4,
			dst_ip        IPv4,
			cnt           SimpleAggregateFunction(sum, UInt64),
			blocked_cnt   SimpleAggregateFunction(sum, UInt64),
			allowed_cnt   SimpleAggregateFunction(sum, UInt64),
			bytes_sent    SimpleAggregateFunction(sum, UInt64),
			bytes_recv    SimpleAggregateFunction(sum, UInt64),
			packets_sent  SimpleAggregateFunction(sum, UInt64),
			packets_recv  SimpleAggregateFunction(sum, UInt64),
			src_lat_sum   SimpleAggregateFunction(sum, Float64),
			src_lon_sum   SimpleAggregateFunction(sum, Float64),
			dst_lat_sum   SimpleAggregateFunction(sum, Float64),
			dst_lon_sum   SimpleAggregateFunction(sum, Float64),
			coord_weight  SimpleAggregateFunction(sum, UInt64),
			last_action   AggregateFunction(argMax, String, DateTime64(3)),
			rule          AggregateFunction(any, String),
			proto         AggregateFunction(any, String),
			src_port      AggregateFunction(any, UInt32),
			dst_port      AggregateFunction(any, UInt32),
			device        AggregateFunction(any, String),
			src_zone      AggregateFunction(any, String),
			dst_zone      AggregateFunction(any, String),
			src_country   AggregateFunction(any, String),
			dst_country   AggregateFunction(any, String),
			src_city      AggregateFunction(any, String),
			dst_city      AggregateFunction(any, String)
		)
		ENGINE = AggregatingMergeTree()
		PARTITION BY %[4]s
		ORDER BY (%[2]s, src_ip, dst_ip)
		TTL %[5]s
		SETTINGS ttl_only_drop_parts = 1
	`, name, timeCol, timeType, partitionBy, ttlExpr)
}

// ipEdgesSelectBody — SELECT для MV и INSERT backfill.
// Колонки квалифицированы traffic_logs.* (подзапрос enrich тоже AS traffic_logs).
func ipEdgesSelectBody(timeExpr, timeAlias string) string {
	coordOK := `(traffic_logs.src_lat != 0 OR traffic_logs.src_lon != 0) AND (traffic_logs.dst_lat != 0 OR traffic_logs.dst_lon != 0)`
	return fmt.Sprintf(`SELECT
			%s AS %s,
			traffic_logs.src_ip AS src_ip,
			traffic_logs.dst_ip AS dst_ip,
			count() AS cnt,
			%s AS blocked_cnt,
			%s AS allowed_cnt,
			sum(traffic_logs.bytes_sent) AS bytes_sent,
			sum(traffic_logs.bytes_recv) AS bytes_recv,
			sum(traffic_logs.packets_sent) AS packets_sent,
			sum(traffic_logs.packets_recv) AS packets_recv,
			sumIf(traffic_logs.src_lat, %s) AS src_lat_sum,
			sumIf(traffic_logs.src_lon, %s) AS src_lon_sum,
			sumIf(traffic_logs.dst_lat, %s) AS dst_lat_sum,
			sumIf(traffic_logs.dst_lon, %s) AS dst_lon_sum,
			sum(if(%s, toUInt64(1), toUInt64(0))) AS coord_weight,
			argMaxState(traffic_logs.action, traffic_logs.timestamp) AS last_action,
			anyState(traffic_logs.rule) AS rule,
			anyState(traffic_logs.proto) AS proto,
			anyState(traffic_logs.src_port) AS src_port,
			anyState(traffic_logs.dst_port) AS dst_port,
			anyState(traffic_logs.device) AS device,
			anyState(traffic_logs.src_zone) AS src_zone,
			anyState(traffic_logs.dst_zone) AS dst_zone,
			anyState(traffic_logs.src_country) AS src_country,
			anyState(traffic_logs.dst_country) AS dst_country,
			anyState(traffic_logs.src_city) AS src_city,
			anyState(traffic_logs.dst_city) AS dst_city`,
		timeExpr, timeAlias,
		sqlclause.SumBlockedSQL(), sqlclause.SumAllowedSQL(),
		coordOK, coordOK, coordOK, coordOK, coordOK)
}

func ipEdgesCreateMVSQL(viewName, destTable, timeExpr, timeAlias string) string {
	return fmt.Sprintf(`
		CREATE MATERIALIZED VIEW %s
		TO %s AS
		%s
		FROM traffic_logs
		GROUP BY %s, src_ip, dst_ip
	`, viewName, destTable, ipEdgesSelectBody(timeExpr, timeAlias), timeAlias)
}
