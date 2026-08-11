package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/model"
)

const trafficLogsIPv4Next = "traffic_logs__ipv4_next"

// EnsureTrafficLogsIPv4 мигрирует src_ip/dst_ip String → IPv4 (ORDER BY keys —
// через recreate + EXCHANGE, не MODIFY COLUMN). Пустой/уже IPv4 том — no-op
// с записью версии. Go model остаётся string; toString() на чтении API.
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
		// Cold bootstrap ещё не создал таблицу (init.sql) — версию не ставим.
		slog.Info("traffic_logs ipv4: table missing, skip")
		return nil
	}

	// Geo-колонки могут отсутствовать на старых томах — DDL next их включает.
	if err := ensureTrafficLogGeoColumns(ctx, ch); err != nil {
		return err
	}

	typ, err := columnType(ctx, ch, "traffic_logs", "src_ip")
	if err != nil {
		return err
	}
	if isIPv4Type(typ) {
		slog.Info("traffic_logs ipv4: already IPv4", "type", typ)
		return setSchemaVersion(ctx, ch, schemaComponentTrafficLogsIP, schemaVersionTrafficLogsIP)
	}

	slog.Info("traffic_logs ipv4: migrating String → IPv4 via EXCHANGE")
	_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+trafficLogsIPv4Next)

	if err := execDDL(ctx, ch, trafficLogsIPv4CreateSQL(trafficLogsIPv4Next)); err != nil {
		return fmt.Errorf("create %s: %w", trafficLogsIPv4Next, err)
	}
	dropNext := func() {
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		_ = ch.Exec(dctx, "DROP TABLE IF EXISTS "+trafficLogsIPv4Next)
	}

	copySQL := fmt.Sprintf(`
		INSERT INTO %s (
			timestamp, parsed_at, ingest_time, vendor, device,
			src_ip, dst_ip, src_port, dst_port, action,
			rule, proto, src_zone, dst_zone, src_country, dst_country,
			src_city, dst_city, src_region, dst_region,
			src_lat, src_lon, dst_lat, dst_lon,
			bytes_sent, bytes_recv, packets_sent, packets_recv, raw
		)
		SELECT
			timestamp, parsed_at, ingest_time, vendor, device,
			toIPv4OrZero(src_ip), toIPv4OrZero(dst_ip), src_port, dst_port, action,
			rule, proto, src_zone, dst_zone, src_country, dst_country,
			src_city, dst_city, src_region, dst_region,
			src_lat, src_lon, dst_lat, dst_lon,
			bytes_sent, bytes_recv, packets_sent, packets_recv, raw
		FROM traffic_logs
	`, trafficLogsIPv4Next)

	ictx, icancel := context.WithTimeout(ctx, 6*time.Hour)
	err = ch.Exec(ictx, copySQL)
	icancel()
	if err != nil {
		dropNext()
		return fmt.Errorf("copy traffic_logs → ipv4: %w", err)
	}

	if err := execDDL(ctx, ch, fmt.Sprintf("EXCHANGE TABLES traffic_logs AND %s", trafficLogsIPv4Next)); err != nil {
		dropNext()
		return fmt.Errorf("exchange traffic_logs: %w", err)
	}
	dropNext()

	if err := setSchemaVersion(ctx, ch, schemaComponentTrafficLogsIP, schemaVersionTrafficLogsIP); err != nil {
		return fmt.Errorf("set traffic_logs_ip schema version: %w", err)
	}
	slog.Info("traffic_logs ipv4: migration done", "version", schemaVersionTrafficLogsIP)
	return nil
}

func trafficLogsIPv4CreateSQL(table string) string {
	return fmt.Sprintf(`
		CREATE TABLE %s
		(
			timestamp     DateTime64(3),
			parsed_at     DateTime64(3) DEFAULT now64(3),
			ingest_time   DateTime64(3) DEFAULT now64(3),
			vendor        LowCardinality(String) DEFAULT '',
			device        String,
			src_ip        IPv4,
			dst_ip        IPv4,
			src_port      UInt32,
			dst_port      UInt32,
			action        LowCardinality(String),
			success       UInt8 MATERIALIZED
			              if(lower(action) IN (%s), 1, 0),
			rule          String,
			proto         LowCardinality(String),
			src_zone      String,
			dst_zone      String,
			src_country   String,
			dst_country   String,
			src_city      String DEFAULT '',
			dst_city      String DEFAULT '',
			src_region    String DEFAULT '',
			dst_region    String DEFAULT '',
			src_lat       Float64 DEFAULT 0,
			src_lon       Float64 DEFAULT 0,
			dst_lat       Float64 DEFAULT 0,
			dst_lon       Float64 DEFAULT 0,
			bytes_sent    UInt64,
			bytes_recv    UInt64,
			packets_sent  UInt64,
			packets_recv  UInt64,
			raw           String CODEC(ZSTD(3)),
			INDEX idx_src_ip      src_ip      TYPE bloom_filter(0.01) GRANULARITY 4,
			INDEX idx_dst_ip      dst_ip      TYPE bloom_filter(0.01) GRANULARITY 4,
			INDEX idx_dst_port    dst_port    TYPE minmax              GRANULARITY 4,
			INDEX idx_action      action      TYPE set(0)              GRANULARITY 4,
			INDEX idx_dst_country dst_country TYPE bloom_filter(0.01) GRANULARITY 4
		)
		ENGINE = MergeTree()
		PARTITION BY toYYYYMMDD(timestamp)
		ORDER BY (timestamp, src_ip, dst_ip, action)
		TTL toDateTime(timestamp) + INTERVAL 30 DAY DELETE
		SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1
	`, table, model.AllowedInClause())
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
