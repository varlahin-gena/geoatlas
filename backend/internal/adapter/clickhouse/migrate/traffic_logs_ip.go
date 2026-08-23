package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"geoatlas/internal/model"
)

const (
	trafficLogsIPv4Next     = "traffic_logs__ipv4_next"
	trafficLogsDesiredOrder = "tostartofhour(timestamp),src_ip,dst_ip"
)

// EnsureTrafficLogsIPv4 мигрирует layout traffic_logs: IPv4, ORDER BY
// (toStartOfHour(timestamp), src_ip, dst_ip), без raw, LC geo/zone.
// Ключи сортировки — через recreate + EXCHANGE, не MODIFY COLUMN.
func EnsureTrafficLogsIPv4(ctx context.Context, ch clickhouse.Conn) error {
	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentTrafficLogsIP, schemaVersionTrafficLogsIP)
	if err != nil {
		return err
	}
	if !needDDL {
		return nil
	}

	exists, err := tableExists(ctx, ch, "traffic_logs")
	if err != nil {
		return err
	}
	if !exists {
		slog.Info("traffic_logs layout: creating table")
		if err := execDDL(ctx, ch, trafficLogsCreateSQL("traffic_logs", true)); err != nil {
			return fmt.Errorf("create traffic_logs: %w", err)
		}
		return setSchemaVersion(ctx, ch, schemaComponentTrafficLogsIP, schemaVersionTrafficLogsIP)
	}

	if err := ensureTrafficLogGeoColumns(ctx, ch); err != nil {
		return err
	}

	needRebuild, reason, err := trafficLogsNeedsRebuild(ctx, ch)
	if err != nil {
		return err
	}
	if !needRebuild {
		slog.Info("traffic_logs layout: already current", "version", schemaVersionTrafficLogsIP)
		return setSchemaVersion(ctx, ch, schemaComponentTrafficLogsIP, schemaVersionTrafficLogsIP)
	}

	slog.Info("traffic_logs layout: migrating via EXCHANGE", "reason", reason)
	_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+trafficLogsIPv4Next)

	if err := execDDL(ctx, ch, trafficLogsIPv4CreateSQL(trafficLogsIPv4Next)); err != nil {
		return fmt.Errorf("create %s: %w", trafficLogsIPv4Next, err)
	}
	dropNext := func() {
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		_ = ch.Exec(dctx, "DROP TABLE IF EXISTS "+trafficLogsIPv4Next)
	}

	srcIP, dstIP := "src_ip", "dst_ip"
	typ, err := columnType(ctx, ch, "traffic_logs", "src_ip")
	if err != nil {
		dropNext()
		return err
	}
	if !isIPv4Type(typ) {
		srcIP = "toIPv4OrZero(src_ip)"
		dstIP = "toIPv4OrZero(dst_ip)"
	}

	copySQL := fmt.Sprintf(`
		INSERT INTO %s (
			timestamp, parsed_at, ingest_time, vendor, device,
			src_ip, dst_ip, src_port, dst_port, action,
			rule, proto, src_zone, dst_zone, src_country, dst_country,
			src_city, dst_city, src_region, dst_region,
			src_lat, src_lon, dst_lat, dst_lon,
			bytes_sent, bytes_recv, packets_sent, packets_recv
		)
		SELECT
			timestamp, parsed_at, ingest_time, vendor, device,
			%s, %s, src_port, dst_port, action,
			rule, proto, src_zone, dst_zone, src_country, dst_country,
			src_city, dst_city, src_region, dst_region,
			src_lat, src_lon, dst_lat, dst_lon,
			bytes_sent, bytes_recv, packets_sent, packets_recv
		FROM traffic_logs
	`, trafficLogsIPv4Next, srcIP, dstIP)

	ictx, icancel := context.WithTimeout(ctx, 6*time.Hour)
	err = ch.Exec(ictx, copySQL)
	icancel()
	if err != nil {
		dropNext()
		return fmt.Errorf("copy traffic_logs → layout: %w", err)
	}

	if err := execDDL(ctx, ch, fmt.Sprintf("EXCHANGE TABLES traffic_logs AND %s", trafficLogsIPv4Next)); err != nil {
		dropNext()
		return fmt.Errorf("exchange traffic_logs: %w", err)
	}
	dropNext()

	if err := setSchemaVersion(ctx, ch, schemaComponentTrafficLogsIP, schemaVersionTrafficLogsIP); err != nil {
		return fmt.Errorf("set traffic_logs_ip schema version: %w", err)
	}
	slog.Info("traffic_logs layout: migration done", "version", schemaVersionTrafficLogsIP, "reason", reason)
	return nil
}

func trafficLogsSuccessExpr() string {
	return fmt.Sprintf("if(lower(action) IN (%s), 1, 0)", model.AllowedInClause())
}

func trafficLogsIPv4CreateSQL(table string) string {
	return trafficLogsCreateSQL(table, false)
}

func trafficLogsCreateSQL(table string, ifNotExists bool) string {
	kw := ""
	if ifNotExists {
		kw = "IF NOT EXISTS "
	}
	return fmt.Sprintf(`
		CREATE TABLE %s%s
		(
			timestamp     DateTime64(3),
			parsed_at     DateTime64(3) DEFAULT now64(3),
			ingest_time   DateTime64(3) DEFAULT now64(3),
			vendor        LowCardinality(String) DEFAULT '',
			device        LowCardinality(String),
			src_ip        IPv4,
			dst_ip        IPv4,
			src_port      UInt32,
			dst_port      UInt32,
			action        LowCardinality(String),
			success       UInt8 MATERIALIZED
			              %s,
			rule          String,
			proto         LowCardinality(String),
			src_zone      LowCardinality(String),
			dst_zone      LowCardinality(String),
			src_country   LowCardinality(String),
			dst_country   LowCardinality(String),
			src_city      LowCardinality(String) DEFAULT '',
			dst_city      LowCardinality(String) DEFAULT '',
			src_region    LowCardinality(String) DEFAULT '',
			dst_region    LowCardinality(String) DEFAULT '',
			src_lat       Float64 DEFAULT 0,
			src_lon       Float64 DEFAULT 0,
			dst_lat       Float64 DEFAULT 0,
			dst_lon       Float64 DEFAULT 0,
			bytes_sent    UInt64,
			bytes_recv    UInt64,
			packets_sent  UInt64,
			packets_recv  UInt64,
			INDEX idx_src_ip      src_ip      TYPE bloom_filter(0.01) GRANULARITY 4,
			INDEX idx_dst_ip      dst_ip      TYPE bloom_filter(0.01) GRANULARITY 4,
			INDEX idx_dst_port    dst_port    TYPE minmax              GRANULARITY 4,
			INDEX idx_action      action      TYPE set(0)              GRANULARITY 4
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMMDD(timestamp)
		ORDER BY (toStartOfHour(timestamp), src_ip, dst_ip)
		TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
		SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
	`, kw, table, trafficLogsSuccessExpr())
}

func trafficLogsNeedsRebuild(ctx context.Context, ch clickhouse.Conn) (bool, string, error) {
	typ, err := columnType(ctx, ch, "traffic_logs", "src_ip")
	if err != nil {
		return false, "", err
	}
	if !isIPv4Type(typ) {
		return true, "src_ip type " + typ, nil
	}
	key, err := tableSortingKey(ctx, ch, "traffic_logs")
	if err != nil {
		return false, "", err
	}
	if normalizeSortingKey(key) != trafficLogsDesiredOrder {
		return true, "sorting_key " + key, nil
	}
	hasRaw, err := columnExists(ctx, ch, "traffic_logs", "raw")
	if err != nil {
		return false, "", err
	}
	if hasRaw {
		return true, "raw column present", nil
	}
	ct, err := columnType(ctx, ch, "traffic_logs", "src_country")
	if err != nil {
		return false, "", err
	}
	if !strings.Contains(ct, "LowCardinality") {
		return true, "src_country type " + ct, nil
	}
	return false, "", nil
}

func columnType(ctx context.Context, ch clickhouse.Conn, table, column string) (string, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var typ string
	err := ch.QueryRow(qctx, `
		SELECT type
		FROM system.columns
		WHERE database = currentDatabase() AND table = {table:String} AND name = {column:String}
		LIMIT 1
	`, clickhouse.Named("table", table), clickhouse.Named("column", column)).Scan(&typ)
	if err != nil {
		return "", fmt.Errorf("column type %s.%s: %w", table, column, err)
	}
	return typ, nil
}

func isIPv4Type(typ string) bool {
	t := strings.TrimSpace(typ)
	return t == "IPv4" || strings.HasPrefix(t, "IPv4(")
}
