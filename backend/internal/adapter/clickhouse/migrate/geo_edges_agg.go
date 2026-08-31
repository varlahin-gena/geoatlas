package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/adapter/clickhouse/aggstate"
	"geoatlas/internal/adapter/clickhouse/query"
	"geoatlas/internal/adapter/clickhouse/sqlclause"
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
		if err := ensureGeoEdgesTable(ctx, ch, "continent"); err != nil {
			return err
		}
		if err := setSchemaVersion(ctx, ch, schemaComponentGeoEdges, schemaVersionGeoEdges); err != nil {
			return fmt.Errorf("set geo_edges schema version: %w", err)
		}
	}
	// Lookup для rebuild без ALTER UPDATE traffic_logs (нужна и на старых томах).
	if err := ensureGeoEnrichIPTable(ctx, ch); err != nil {
		return err
	}

	slog.Info("geo edges agg: schema ready")
	return nil
}

// BackfillGeoEdgesAgg дозаполняет city/country/continent daily-агрегаты и включает PreferGeoEdgesAgg.
func BackfillGeoEdgesAgg(ctx context.Context, ch clickhouse.Conn) error {
	for _, groupBy := range []string{"city", "country", "continent"} {
		if err := backfillGeoEdgesAgg(ctx, ch, groupBy); err != nil {
			aggstate.SetGeoEdgesAggReady(false)
			return err
		}
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
	okContinent, err := geoEdgesAggReady(ctx, ch, "continent")
	if err != nil {
		aggstate.SetGeoEdgesAggReady(false)
		return err
	}
	ready := okCity && okCountry && okContinent
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
	exists, err := tableExists(ctx, ch, table)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, nil
	}
	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		return false, err
	}
	if rawRows == 0 {
		return true, nil
	}
	missing, err := missingClosedPartitionDays(ctx, ch, "traffic_logs", table)
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

func ensureGeoEnrichIPTable(ctx context.Context, ch clickhouse.Conn) error {
	exists, err := tableExists(ctx, ch, sqlclause.GeoEnrichIPTable)
	if err != nil {
		return err
	}
	if exists {
		typ, err := columnType(ctx, ch, sqlclause.GeoEnrichIPTable, "ip")
		if err != nil {
			return err
		}
		if !isIPv4Type(typ) {
			if err := execDDL(ctx, ch, "DROP TABLE IF EXISTS "+sqlclause.GeoEnrichIPTable); err != nil {
				return fmt.Errorf("drop old %s: %w", sqlclause.GeoEnrichIPTable, err)
			}
			exists = false
		}
	}
	if exists {
		return nil
	}
	q := fmt.Sprintf(`
		CREATE TABLE %s
		(
			ip       IPv4,
			country  String,
			region   String,
			city     String,
			lat      Float64,
			lon      Float64
		)
		ENGINE = MergeTree()
		ORDER BY ip
	`, sqlclause.GeoEnrichIPTable)
	if err := execDDL(ctx, ch, q); err != nil {
		return fmt.Errorf("create %s: %w", sqlclause.GeoEnrichIPTable, err)
	}
	return nil
}

// geoEdgesEnrichedFromSQL — обёртка над sqlclause.TrafficLogsEnrichedFromSQL (rebuild INSERT SELECT).
func geoEdgesEnrichedFromSQL(wherePred string) string {
	return sqlclause.TrafficLogsEnrichedFromSQL(wherePred)
}

func ensureGeoEdgesTable(ctx context.Context, ch clickhouse.Conn, groupBy string) error {
	table := sqlclause.GeoEdgesTable(groupBy)
	mv := sqlclause.GeoEdgesMV(groupBy)
	if table == "" || mv == "" {
		return fmt.Errorf("geo edges: invalid groupBy %q", groupBy)
	}
	next := table + "__next"

	createTable := func(name string) string {
		return fmt.Sprintf(`
		CREATE TABLE %s
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
		PARTITION BY day
		ORDER BY (day, src_key, dst_key)
		TTL day + INTERVAL 30 DAY DELETE
		SETTINGS ttl_only_drop_parts = 1
	`, name)
	}

	exists, err := tableExists(ctx, ch, table)
	if err != nil {
		return err
	}
	if !exists {
		if err := execDDL(ctx, ch, createTable(table)); err != nil {
			return fmt.Errorf("create %s: %w", table, err)
		}
	} else {
		pk, err := tablePartitionKey(ctx, ch, table)
		if err != nil {
			return err
		}
		if !isDayPartitionKey(pk) {
			_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+next)
			if err := execDDL(ctx, ch, createTable(next)); err != nil {
				return fmt.Errorf("create %s: %w", next, err)
			}
			if err := execDDL(ctx, ch, fmt.Sprintf("EXCHANGE TABLES %s AND %s", table, next)); err != nil {
				_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+next)
				return fmt.Errorf("exchange %s: %w", table, err)
			}
			_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+next)
			slog.Info("geo edges agg: table rebuilt", "table", table, "reason", "partition_key "+pk, "note", "backfill required")
		} else if err := ensureTTLOnlyDropPartsSetting(ctx, ch, table); err != nil {
			return err
		}
	}

	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprsPrefixed("traffic_logs", groupBy)
	coordOK := sqlclause.GeoCoordOK
	selectBody := geoEdgesAggSelectBody(srcKey, dstKey, srcLabel, dstLabel, coordOK)

	createMV := func(viewName string) string {
		return fmt.Sprintf(`
		CREATE MATERIALIZED VIEW %s
		TO %s AS
		%s
		FROM traffic_logs
		GROUP BY day, src_key, dst_key
	`, viewName, table, selectBody)
	}
	if err := replaceMaterializedView(ctx, ch, mv, createMV); err != nil {
		return err
	}
	return nil
}

// geoEdgesAggSelectBody — SELECT-список для MV и backfill INSERT.
// Колонки src_city/country квалифицируются как traffic_logs.*: иначе CH
// подставляет anyState(...) AS src_city в trimBoth(src_city) (code 43).
func geoEdgesAggSelectBody(srcKey, dstKey, srcLabel, dstLabel, coordOK string) string {
	return fmt.Sprintf(`SELECT
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
			anyState(traffic_logs.src_country) AS src_country,
			anyState(traffic_logs.dst_country) AS dst_country,
			anyState(traffic_logs.src_city) AS src_city,
			anyState(traffic_logs.dst_city) AS dst_city`,
		srcKey, dstKey, sqlclause.SumBlockedSQL(), sqlclause.SumAllowedSQL(),
		coordOK, coordOK, coordOK, coordOK, sqlclause.CoordWeightSQL(),
		srcLabel, dstLabel)
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
	missing, err := missingClosedPartitionDays(ctx, ch, "traffic_logs", table)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		slog.Info("geo edges agg: up to date", "group_by", groupBy, "rows", aggRows)
		return nil
	}
	slog.Info("geo edges agg: backfill started", "group_by", groupBy, "days", len(missing))
	return insertGeoEdgesDays(ctx, ch, groupBy, missing)
}

func insertGeoEdgesDays(ctx context.Context, ch clickhouse.Conn, groupBy string, days []time.Time) error {
	table := sqlclause.GeoEdgesTable(groupBy)
	srcKey, dstKey, srcLabel, dstLabel := sqlclause.GeoGroupExprsPrefixed("traffic_logs", groupBy)
	selectBody := geoEdgesAggSelectBody(srcKey, dstKey, srcLabel, dstLabel, sqlclause.GeoCoordOK)
	// Plain traffic_logs: enrich JOIN OOMs on small CH hosts; map overlays ga_geo_enrich_ip on read.
	fromSQL := fmt.Sprintf("FROM traffic_logs\n\t\tWHERE %s", sqlclause.HourTimestampRangeSQL("traffic_logs.timestamp"))

	insertTpl := fmt.Sprintf(`
		INSERT INTO %s
		%s
		%s
		GROUP BY day, src_key, dst_key
		%s
	`, table, selectBody, fromSQL, query.BackfillAggSettings())

	for i, day := range days {
		if err := ctx.Err(); err != nil {
			return err
		}
		dayStart := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
		for h := 0; h < 24; h++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			hourStart := dayStart.Add(time.Duration(h) * time.Hour)
			ictx, icancel := context.WithTimeout(ctx, 30*time.Minute)
			err := ch.Exec(ictx, insertTpl, hourStart, hourStart)
			icancel()
			if err != nil {
				return fmt.Errorf("geo edges backfill %s day %s hour %02d: %w", groupBy, day.Format("2006-01-02"), h, err)
			}
		}
		slog.Info("geo edges agg: backfill day", "group_by", groupBy, "done", i+1, "total", len(days), "day", day.Format("2006-01-02"))
	}
	return nil
}

// RebuildGeoEdgesLookback пересобирает daily/hourly агрегаты за закрытые дни lookback
// после EnrichLogsMissingGeo. Сегодня пишет только MV (ingest geo).
// lookbackDays <= 0 — все закрытые дни из system.parts.
func RebuildGeoEdgesLookback(ctx context.Context, ch clickhouse.Conn, lookbackDays int) error {
	if ch == nil {
		return nil
	}
	days, err := listLookbackClosedDays(ctx, ch, lookbackDays)
	if err != nil {
		return err
	}
	if len(days) == 0 {
		slog.Info("geo edges agg: rebuild skipped, no closed days")
		return nil
	}
	for _, groupBy := range []string{"city", "country", "continent"} {
		if err := rebuildGeoEdgesDays(ctx, ch, groupBy, days); err != nil {
			return err
		}
	}
	if err := rebuildIPEdgesDays(ctx, ch, sqlclause.IPEdgesDailyTable, days); err != nil {
		return err
	}
	if err := rebuildIPEdgesDays(ctx, ch, sqlclause.IPEdgesHourlyTable, days); err != nil {
		return err
	}
	slog.Info("geo edges agg: rebuild done", "days", len(days), "lookback_days", lookbackDays)
	return nil
}

func listLookbackClosedDays(ctx context.Context, ch clickhouse.Conn, lookbackDays int) ([]time.Time, error) {
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	lookbackPred := ""
	if lookbackDays > 0 {
		lookbackPred = fmt.Sprintf("AND d >= today() - INTERVAL %d DAY", lookbackDays)
	}
	rows, err := ch.Query(qctx, fmt.Sprintf(`
		SELECT d FROM (
			SELECT DISTINCT %s AS d
			FROM system.parts
			WHERE database = currentDatabase() AND table = 'traffic_logs' AND active
		)
		WHERE d < today() AND d > toDate('2000-01-01') %s
		ORDER BY d DESC
	`, partitionDayExpr(), lookbackPred))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDays(rows)
}

func rebuildGeoEdgesDays(ctx context.Context, ch clickhouse.Conn, groupBy string, days []time.Time) error {
	if len(days) == 0 {
		return nil
	}
	table := sqlclause.GeoEdgesTable(groupBy)
	if table == "" {
		return fmt.Errorf("geo edges rebuild: invalid groupBy %q", groupBy)
	}
	for _, day := range days {
		if err := dropDatePartition(ctx, ch, table, day); err != nil {
			return fmt.Errorf("geo edges rebuild drop %s: %w", groupBy, err)
		}
	}
	return insertGeoEdgesDays(ctx, ch, groupBy, days)
}
