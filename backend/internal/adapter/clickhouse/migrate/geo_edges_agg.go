package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/aggstate"
	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/adapter/clickhouse/sqlclause"
)

// EnsureGeoEdgesAggSchema добавляет geo-колонки и создаёт daily-таблицы/MV без backfill.
func EnsureGeoEdgesAggSchema(ctx context.Context, ch clickhouse.Conn) error {
	slog.Info("geo edges agg: setup started")
	aggstate.SetGeoEdgesAggReady(false)

	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentGeoEdges, schemaVersionGeoEdges)
	if err != nil {
		return err
	}
	if needDDL {
		if err := ensureTrafficLogGeoColumns(ctx, ch); err != nil {
			return err
		}
		if err := ensureGeoEdgesTable(ctx, ch, "city"); err != nil {
			return err
		}
		if err := ensureGeoEdgesTable(ctx, ch, "country"); err != nil {
			return err
		}
		if err := setSchemaVersion(ctx, ch, schemaComponentGeoEdges, schemaVersionGeoEdges); err != nil {
			return fmt.Errorf("set geo_edges schema version: %w", err)
		}
	}

	slog.Info("geo edges agg: schema ready")
	return nil
}

// EnsureGeoEdgesAgg — schema + backfill city/country.
func EnsureGeoEdgesAgg(ctx context.Context, ch clickhouse.Conn) error {
	if err := EnsureGeoEdgesAggSchema(ctx, ch); err != nil {
		return err
	}
	return BackfillGeoEdgesAgg(ctx, ch)
}

// BackfillGeoEdgesAgg дозаполняет city/country daily-агрегаты и включает PreferGeoEdgesAgg.
func BackfillGeoEdgesAgg(ctx context.Context, ch clickhouse.Conn) error {
	if err := backfillGeoEdgesAgg(ctx, ch, "city"); err != nil {
		aggstate.SetGeoEdgesAggReady(false)
		return err
	}
	if err := backfillGeoEdgesAgg(ctx, ch, "country"); err != nil {
		aggstate.SetGeoEdgesAggReady(false)
		return err
	}
	aggstate.SetGeoEdgesAggReady(true)
	slog.Info("geo edges agg: ready")
	return nil
}

// RefreshGeoEdgesAggReady включает PreferGeoEdgesAgg, если missing days уже нет.
func RefreshGeoEdgesAggReady(ctx context.Context, ch clickhouse.Conn) error {
	okCity, err := geoEdgesAggReady(ctx, ch, "city")
	if err != nil {
		aggstate.SetGeoEdgesAggReady(false)
		return err
	}
	okCountry, err := geoEdgesAggReady(ctx, ch, "country")
	if err != nil {
		aggstate.SetGeoEdgesAggReady(false)
		return err
	}
	ready := okCity && okCountry
	aggstate.SetGeoEdgesAggReady(ready)
	if ready {
		slog.Info("geo edges agg: already up to date")
	} else {
		slog.Info("geo edges agg: backfill pending")
	}
	return nil
}

func geoEdgesAggReady(ctx context.Context, ch clickhouse.Conn, groupBy string) (bool, error) {
	table := sqlclause.GeoEdgesTable(groupBy)
	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		return false, err
	}
	if rawRows == 0 {
		return true, nil
	}
	missing, err := missingGeoDays(ctx, ch, table)
	if err != nil {
		return false, err
	}
	return len(missing) == 0, nil
}

func ensureTrafficLogGeoColumns(ctx context.Context, ch clickhouse.Conn) error {
	alters := []string{
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_city String DEFAULT ''`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_city String DEFAULT ''`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_region String DEFAULT ''`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_region String DEFAULT ''`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_lat Float64 DEFAULT 0`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS src_lon Float64 DEFAULT 0`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_lat Float64 DEFAULT 0`,
		`ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS dst_lon Float64 DEFAULT 0`,
	}
	for _, q := range alters {
		if err := execDDL(ctx, ch, q); err != nil {
			return fmt.Errorf("geo columns: %w", err)
		}
	}
	return nil
}

func ensureGeoEdgesTable(ctx context.Context, ch clickhouse.Conn, groupBy string) error {
	table := sqlclause.GeoEdgesTable(groupBy)
	mv := sqlclause.GeoEdgesMV(groupBy)

	if err := execDDL(ctx, ch, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s
		(
			day           Date,
			src_key       String,
			dst_key       String,
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
			src_label     AggregateFunction(any, String),
			dst_label     AggregateFunction(any, String),
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
		PARTITION BY toYYYYMM(day)
		ORDER BY (day, src_key, dst_key)
		TTL day + INTERVAL 30 DAY DELETE
	`, table)); err != nil {
		return fmt.Errorf("create %s: %w", table, err)
	}

	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs(groupBy)
	coordOK := sqlclause.GeoCoordOK

	createMV := func(viewName string) string {
		return fmt.Sprintf(`
		CREATE MATERIALIZED VIEW %s
		TO %s AS
		SELECT
			toDate(timestamp) AS day,
			%s AS src_key,
			%s AS dst_key,
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
			anyState(%s) AS src_label,
			anyState(%s) AS dst_label,
			argMaxState(action, timestamp) AS last_action,
			anyState(rule) AS rule,
			anyState(proto) AS proto,
			anyState(src_port) AS src_port,
			anyState(dst_port) AS dst_port,
			anyState(device) AS device,
			anyState(src_zone) AS src_zone,
			anyState(dst_zone) AS dst_zone,
			anyState(src_country) AS src_country,
			anyState(dst_country) AS dst_country,
			anyState(src_city) AS src_city,
			anyState(dst_city) AS dst_city
		FROM traffic_logs
		GROUP BY day, src_key, dst_key
	`, viewName, table, srcKey, dstKey, sqlclause.SumBlockedSQL(), sqlclause.SumAllowedSQL(),
			coordOK, coordOK, coordOK, coordOK, sqlclause.CoordWeightSQL(),
			srcLabel, dstLabel)
	}
	if err := replaceMaterializedView(ctx, ch, mv, createMV); err != nil {
		return err
	}
	return nil
}

func backfillGeoEdgesAgg(ctx context.Context, ch clickhouse.Conn, groupBy string) error {
	table := sqlclause.GeoEdgesTable(groupBy)

	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		return fmt.Errorf("count traffic_logs: %w", err)
	}
	if rawRows == 0 {
		slog.Info("geo edges agg: traffic_logs empty", "group_by", groupBy)
		return nil
	}
	aggRows, err := countTableRows(ctx, ch, table)
	if err != nil {
		return err
	}
	if aggRows > 0 {
		// Частичный backfill как у IP-edges: missing days.
		missing, err := missingGeoDays(ctx, ch, table)
		if err != nil || len(missing) == 0 {
			slog.Info("geo edges agg: up to date", "group_by", groupBy, "rows", aggRows)
			return err
		}
		return insertGeoEdgesDays(ctx, ch, groupBy, missing)
	}
	days, err := listAllRawLogDays(ctx, ch)
	if err != nil {
		return err
	}
	slog.Info("geo edges agg: backfill started", "group_by", groupBy, "days", len(days))
	return insertGeoEdgesDays(ctx, ch, groupBy, days)
}

func listAllRawLogDays(ctx context.Context, ch clickhouse.Conn) ([]time.Time, error) {
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rows, err := ch.Query(qctx, `
		SELECT DISTINCT toDate(timestamp) AS d FROM traffic_logs ORDER BY d DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var days []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

func missingGeoDays(ctx context.Context, ch clickhouse.Conn, table string) ([]time.Time, error) {
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rows, err := ch.Query(qctx, fmt.Sprintf(`
		SELECT d FROM (
			SELECT DISTINCT toDate(timestamp) AS d FROM traffic_logs
		) AS raw
		LEFT ANTI JOIN (
			SELECT DISTINCT day AS d FROM %s
		) AS agg USING (d)
		ORDER BY d DESC
	`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var days []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

func insertGeoEdgesDays(ctx context.Context, ch clickhouse.Conn, groupBy string, days []time.Time) error {
	table := sqlclause.GeoEdgesTable(groupBy)
	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprs(groupBy)
	coordOK := sqlclause.GeoCoordOK

	insertTpl := fmt.Sprintf(`
		INSERT INTO %s
		SELECT
			toDate(timestamp) AS day,
			%s AS src_key,
			%s AS dst_key,
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
			anyState(%s) AS src_label,
			anyState(%s) AS dst_label,
			argMaxState(action, timestamp) AS last_action,
			anyState(rule) AS rule,
			anyState(proto) AS proto,
			anyState(src_port) AS src_port,
			anyState(dst_port) AS dst_port,
			anyState(device) AS device,
			anyState(src_zone) AS src_zone,
			anyState(dst_zone) AS dst_zone,
			anyState(src_country) AS src_country,
			anyState(dst_country) AS dst_country,
			anyState(src_city) AS src_city,
			anyState(dst_city) AS dst_city
		FROM traffic_logs
		WHERE toDate(timestamp) = ?
		GROUP BY day, src_key, dst_key
		%s
	`, table, srcKey, dstKey, sqlclause.SumBlockedSQL(), sqlclause.SumAllowedSQL(),
		coordOK, coordOK, coordOK, coordOK, sqlclause.CoordWeightSQL(),
		srcLabel, dstLabel, query.AggSettings())

	for i, day := range days {
		if err := ctx.Err(); err != nil {
			return err
		}
		ictx, icancel := context.WithTimeout(ctx, 30*time.Minute)
		err := ch.Exec(ictx, insertTpl, day)
		icancel()
		if err != nil {
			return fmt.Errorf("geo edges backfill %s day %s: %w", groupBy, day.Format("2006-01-02"), err)
		}
		slog.Info("geo edges agg: backfill day", "group_by", groupBy, "done", i+1, "total", len(days), "day", day.Format("2006-01-02"))
	}
	return nil
}

// RebuildGeoEdgesLookback пересобирает traffic_edges_{city,country}_daily за окно
// lookback после EnrichLogsMissingGeo: MV пишет только новые INSERT, UPDATE логов
// сам агрегат не обновляет. lookbackDays <= 0 — все дни из traffic_logs.
func RebuildGeoEdgesLookback(ctx context.Context, ch clickhouse.Conn, lookbackDays int) error {
	if ch == nil {
		return nil
	}
	days, err := listLookbackRawLogDays(ctx, ch, lookbackDays)
	if err != nil {
		return err
	}
	if len(days) == 0 {
		slog.Info("geo edges agg: rebuild skipped, no days")
		return nil
	}
	for _, groupBy := range []string{"city", "country"} {
		if err := rebuildGeoEdgesDays(ctx, ch, groupBy, days); err != nil {
			return err
		}
	}
	slog.Info("geo edges agg: rebuild done", "days", len(days), "lookback_days", lookbackDays)
	return nil
}

func listLookbackRawLogDays(ctx context.Context, ch clickhouse.Conn, lookbackDays int) ([]time.Time, error) {
	if lookbackDays <= 0 {
		return listAllRawLogDays(ctx, ch)
	}
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	rows, err := ch.Query(qctx, fmt.Sprintf(`
		SELECT DISTINCT toDate(timestamp) AS d
		FROM traffic_logs
		WHERE timestamp >= now64(3) - INTERVAL %d DAY
		ORDER BY d DESC
	`, lookbackDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var days []time.Time
	for rows.Next() {
		var d time.Time
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		days = append(days, d)
	}
	return days, rows.Err()
}

func rebuildGeoEdgesDays(ctx context.Context, ch clickhouse.Conn, groupBy string, days []time.Time) error {
	if len(days) == 0 {
		return nil
	}
	table := sqlclause.GeoEdgesTable(groupBy)
	literals := make([]string, 0, len(days))
	for _, d := range days {
		literals = append(literals, fmt.Sprintf("toDate('%s')", d.UTC().Format("2006-01-02")))
	}
	del := fmt.Sprintf(`
		ALTER TABLE %s DELETE WHERE day IN (%s)
		SETTINGS mutations_sync = 1
	`, table, strings.Join(literals, ","))
	dctx, dcancel := context.WithTimeout(ctx, 30*time.Minute)
	err := ch.Exec(dctx, del)
	dcancel()
	if err != nil {
		return fmt.Errorf("geo edges rebuild delete %s: %w", groupBy, err)
	}
	return insertGeoEdgesDays(ctx, ch, groupBy, days)
}
