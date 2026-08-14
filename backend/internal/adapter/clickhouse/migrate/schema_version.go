package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Schema version table: bump the constant when Ensure* DDL/MV changes.
const schemaComponentEdgesAgg = "edges_agg"
const schemaComponentGeoEdges = "geo_edges_agg"
const schemaComponentTrafficLogsIP = "traffic_logs_ip"
const schemaComponentHourlyEdges = "hourly_edges_agg"

const schemaVersionEdgesAgg uint32 = 4      // IP daily + coords
const schemaVersionGeoEdges uint32 = 4      // PARTITION BY day
const schemaVersionTrafficLogsIP uint32 = 2 // ORDER BY hour bucket, drop raw, LC geo
const schemaVersionHourlyEdges uint32 = 1

func ensureSchemaVersionTable(ctx context.Context, ch clickhouse.Conn) error {
	return execDDL(ctx, ch, `
		CREATE TABLE IF NOT EXISTS nm_schema_version
		(
			component String,
			version   UInt32,
			updated_at DateTime64(3) DEFAULT now64(3)
		)
		ENGINE = ReplacingMergeTree(updated_at)
		ORDER BY component
	`)
}

func schemaVersion(ctx context.Context, ch clickhouse.Conn, component string) (uint32, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var v uint32
	err := ch.QueryRow(qctx, `
		SELECT version
		FROM nm_schema_version
		WHERE component = ?
		ORDER BY updated_at DESC
		LIMIT 1
	`, component).Scan(&v)
	if err != nil {
		if isNoSchemaVersionRow(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read schema version %q: %w", component, err)
	}
	return v, nil
}

func isNoSchemaVersionRow(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no rows in result set") ||
		strings.Contains(msg, "empty result")
}

func setSchemaVersion(ctx context.Context, ch clickhouse.Conn, component string, version uint32) error {
	qctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return ch.Exec(qctx, `
		INSERT INTO nm_schema_version (component, version, updated_at)
		VALUES (?, ?, now64(3))
	`, component, version)
}

var needsSchemaDDLFn = needsSchemaDDL

func needsSchemaDDL(ctx context.Context, ch clickhouse.Conn, component string, desired uint32) (bool, error) {
	if err := ensureSchemaVersionTable(ctx, ch); err != nil {
		return false, fmt.Errorf("ensure schema version table: %w", err)
	}
	v, err := schemaVersion(ctx, ch, component)
	if err != nil {
		return false, err
	}
	if v >= desired {
		slog.Info("schema: up to date", "component", component, "version", v)
		return false, nil
	}
	slog.Info("schema: DDL needed", "component", component, "have", v, "want", desired)
	return true, nil
}
