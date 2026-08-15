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

// ScanRawAggs читает предварительно агрегированные пары src/dst из
// traffic_edges_daily только когда backfill помечен ready. Частичный
// агрегат даёт дыры на карте — до готовности читаем сырой traffic_logs.
func ScanRawAggs(ctx context.Context, ch clickhouse.Conn, days, limit int, filter string, timeout time.Duration) ([]model.RawAgg, error) {
	if aggstate.AggFromContext(ctx).PreferDailyEdgesAgg() {
		return scanEdgesAgg(ctx, ch, days, limit, filter, timeout)
	}
	return scanRawLogs(ctx, ch, days, limit, filter, timeout)
}

func scanEdgesAgg(ctx context.Context, ch clickhouse.Conn, days, limit int, filter string, timeout time.Duration) ([]model.RawAgg, error) {
	return scanEdgesDailyAsRaw(ctx, ch, TablesOf(ctx).EdgesDaily, days, MapSelect{Limit: limit, Filter: filter}, timeout)
}

func scanEdgesDailyAsRaw(ctx context.Context, ch clickhouse.Conn, table string, days int, sel MapSelect, timeout time.Duration) ([]model.RawAgg, error) {
	if days < 1 {
		days = 1
	}
	if table == "" {
		table = "traffic_edges_daily"
	}
	scopeHaving, scopeArgs := sel.scope().IPAggHavingExpr()
	havingClause := sqlclause.JoinHaving(sqlclause.HavingAggFilterSQL(sel.Filter), scopeHaving)
	query := fmt.Sprintf(`
		SELECT
			toString(src_ip) AS src_ip,
			toString(dst_ip) AS dst_ip,
			sum(cnt) AS cnt,
			sum(blocked_cnt) AS blocked_cnt,
			sum(allowed_cnt) AS allowed_cnt,
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
			anyMerge(dst_city) AS dst_city,
			if(sum(coord_weight) = 0, 0., sum(src_lat_sum) / sum(coord_weight)) AS out_src_lat,
			if(sum(coord_weight) = 0, 0., sum(src_lon_sum) / sum(coord_weight)) AS out_src_lon,
			if(sum(coord_weight) = 0, 0., sum(dst_lat_sum) / sum(coord_weight)) AS out_dst_lat,
			if(sum(coord_weight) = 0, 0., sum(dst_lon_sum) / sum(coord_weight)) AS out_dst_lon,
			sum(bytes_sent) AS bytes_sent,
			sum(bytes_recv) AS bytes_recv,
			sum(packets_sent) AS packets_sent,
			sum(packets_recv) AS packets_recv
		FROM %s
		WHERE day >= today() - ?
		GROUP BY src_ip, dst_ip
		%s
		%s
		%s
		%s
	`, table, havingClause, sqlclause.OrderByAggFilterSQL(sel.Filter), limitClause(sel.Limit), AggSettings())

	qctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]any{days}, scopeArgs...)
	rows, err := ch.Query(qctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRawAggRows(rows)
}

func scanRawLogs(ctx context.Context, ch clickhouse.Conn, days, limit int, filter string, timeout time.Duration) ([]model.RawAgg, error) {
	// Один GROUP BY + LIMIT в CH — без загрузки всех пар дня в Go map.
	return scanRawLogsRelative(ctx, ch, "days", days, MapSelect{Limit: limit, Filter: filter}, timeout)
}

func scanRawAggRows(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.RawAgg, error) {
	var out []model.RawAgg
	for rows.Next() {
		var r model.RawAgg
		if err := rows.Scan(
			&r.SrcIP, &r.DstIP, &r.Count, &r.BlockedCnt, &r.AllowedCnt,
			&r.LastAction, &r.Rule, &r.Proto, &r.SrcPort, &r.DstPort,
			&r.Device, &r.SrcZone, &r.DstZone, &r.SrcCountry, &r.DstCountry,
			&r.SrcCity, &r.DstCity, &r.SrcLat, &r.SrcLon, &r.DstLat, &r.DstLon,
			&r.BytesSent, &r.BytesRecv, &r.PacketsSent, &r.PacketsRecv,
		); err != nil {
			slog.Warn("ScanRawAggs: row scan failed, skipping row", "err", err)
			continue
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
