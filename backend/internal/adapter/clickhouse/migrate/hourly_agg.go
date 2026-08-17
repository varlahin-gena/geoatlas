package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse/aggstate"
	"network_monitor/internal/adapter/clickhouse/sqlclause"
)

// EnsureHourlyEdgesAggSchema создаёт hourly IP-агрегат без backfill.
func EnsureHourlyEdgesAggSchema(ctx context.Context, ch clickhouse.Conn) error {
	slog.Info("hourly edges agg: setup started")
	aggstate.SetHourlyEdgesAggReady(false)

	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentHourlyEdges, schemaVersionHourlyEdges)
	if err != nil {
		return err
	}
	if needDDL {
		if err := applyHourlyEdgesSchema(ctx, ch); err != nil {
			return err
		}
		if err := setSchemaVersion(ctx, ch, schemaComponentHourlyEdges, schemaVersionHourlyEdges); err != nil {
			return fmt.Errorf("set hourly_edges schema version: %w", err)
		}
	}
	slog.Info("hourly edges agg: schema ready")
	return nil
}

func applyHourlyEdgesSchema(ctx context.Context, ch clickhouse.Conn) error {
	const table = sqlclause.IPEdgesHourlyTable
	const next = "traffic_edges_hourly__next"
	createTable := func(name string) string {
		return ipEdgesCreateTableSQL(name, "hour", "DateTime", "toYYYYMMDD(hour)", ipEdgesHourlyTTL)
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
		needRebuild, reason, err := hourlyNeedsRebuild(ctx, ch, table)
		if err != nil {
			return err
		}
		if needRebuild {
			_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+next)
			if err := execDDL(ctx, ch, createTable(next)); err != nil {
				return fmt.Errorf("create %s: %w", next, err)
			}
			if err := execDDL(ctx, ch, fmt.Sprintf("EXCHANGE TABLES %s AND %s", table, next)); err != nil {
				_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+next)
				return fmt.Errorf("exchange %s: %w", table, err)
			}
			_ = execDDL(ctx, ch, "DROP TABLE IF EXISTS "+next)
			slog.Info("hourly edges agg: table rebuilt", "reason", reason, "note", "backfill required")
		} else if err := ensureTTLOnlyDropPartsSetting(ctx, ch, table); err != nil {
			return err
		}
	}

	createMV := func(viewName string) string {
		return ipEdgesCreateMVSQL(viewName, table, "toStartOfHour(traffic_logs.timestamp)", "hour")
	}
	return replaceMaterializedView(ctx, ch, sqlclause.IPEdgesHourlyMV, createMV)
}

func hourlyNeedsRebuild(ctx context.Context, ch clickhouse.Conn, table string) (bool, string, error) {
	typ, err := columnType(ctx, ch, table, "src_ip")
	if err != nil {
		return false, "", err
	}
	if !isIPv4Type(typ) {
		return true, "src_ip type " + typ, nil
	}
	ok, err := columnExists(ctx, ch, table, "coord_weight")
	if err != nil {
		return false, "", err
	}
	if !ok {
		return true, "missing coord_weight", nil
	}
	return false, "", nil
}

func hourlyEdgesReady(ctx context.Context, ch clickhouse.Conn) (bool, error) {
	exists, err := tableExists(ctx, ch, sqlclause.IPEdgesHourlyTable)
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
	missing, err := missingClosedPartitionDaysSince(ctx, ch, "traffic_logs", sqlclause.IPEdgesHourlyTable, 7)
	if err != nil {
		return false, err
	}
	return len(missing) == 0, nil
}

// BackfillHourlyEdgesAgg дозаполняет закрытые дни (today — только MV).
func BackfillHourlyEdgesAgg(ctx context.Context, ch clickhouse.Conn) error {
	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		aggstate.SetHourlyEdgesAggReady(false)
		return fmt.Errorf("count traffic_logs: %w", err)
	}
	if rawRows == 0 {
		aggstate.SetHourlyEdgesAggReady(true)
		slog.Info("hourly edges agg: traffic_logs empty")
		return nil
	}
	days, err := missingClosedPartitionDaysSince(ctx, ch, "traffic_logs", sqlclause.IPEdgesHourlyTable, 7)
	if err != nil {
		aggstate.SetHourlyEdgesAggReady(false)
		return err
	}
	if len(days) == 0 {
		aggstate.SetHourlyEdgesAggReady(true)
		slog.Info("hourly edges agg: already up to date")
		return nil
	}
	slog.Info("hourly edges agg: backfill started", "days", len(days))
	if err := insertIPEdgesDays(ctx, ch, sqlclause.IPEdgesHourlyTable, days, nil); err != nil {
		aggstate.SetHourlyEdgesAggReady(false)
		return err
	}
	aggstate.SetHourlyEdgesAggReady(true)
	slog.Info("hourly edges agg: backfill complete", "days", len(days))
	return nil
}

// RefreshHourlyEdgesAggReady включает PreferHourlyEdgesAgg без INSERT.
func RefreshHourlyEdgesAggReady(ctx context.Context, ch clickhouse.Conn) error {
	ok, err := hourlyEdgesReady(ctx, ch)
	if err != nil {
		aggstate.SetHourlyEdgesAggReady(false)
		return err
	}
	aggstate.SetHourlyEdgesAggReady(ok)
	if ok {
		slog.Info("hourly edges agg: already up to date")
	} else {
		slog.Info("hourly edges agg: backfill pending")
	}
	return nil
}
