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

// EnsureEdgesAggSchema создаёт таблицу/MV предварительной агрегации без backfill.
func EnsureEdgesAggSchema(ctx context.Context, ch clickhouse.Conn) error {
	slog.Info("edges agg: setup started")
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "running", Phase: aggstate.PhaseSchema, Message: "checking schema",
	})

	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentEdgesAgg, schemaVersionEdgesAgg)
	if err != nil {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "error", Message: err.Error()})
		return err
	}
	if needDDL {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "running", Phase: aggstate.PhaseSchema,
			Message:   "rebuilding schema (create MV → EXCHANGE) — map uses traffic_logs until ready",
			StartedAt: time.Now().UTC(),
		})
		if err := applyEdgesAggSchema(ctx, ch); err != nil {
			aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "error", Phase: aggstate.PhaseSchema, Message: err.Error()})
			return err
		}
		if err := setSchemaVersion(ctx, ch, schemaComponentEdgesAgg, schemaVersionEdgesAgg); err != nil {
			return fmt.Errorf("set edges_agg schema version: %w", err)
		}
	}

	slog.Info("edges agg: schema ready")
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "pending", Phase: "", Message: "schema ready; backfill pending",
	})
	return nil
}

// RefreshEdgesAggReady помечает агрегаты ready без INSERT, если закрытые дни догнаны.
// Сегодня пишет только MV. Иначе State=pending (карта читает traffic_logs).
func RefreshEdgesAggReady(ctx context.Context, ch clickhouse.Conn) error {
	logRows, _ := countTableRows(ctx, ch, "traffic_logs")
	aggRows, _ := countTableRows(ctx, ch, sqlclause.IPEdgesDailyTable)
	ready, err := edgesAggReady(ctx, ch)
	if err != nil {
		return err
	}
	if ready {
		msg := "up to date"
		if logRows == 0 {
			msg = "traffic_logs empty"
		}
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "ready", Message: msg, RawRows: logRows, AggRows: aggRows,
		})
		return nil
	}
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "pending", Message: "backfill needed (SKIP_STARTUP_BACKFILL or incomplete)",
		RawRows: logRows, AggRows: aggRows,
	})
	return nil
}

func applyEdgesAggSchema(ctx context.Context, ch clickhouse.Conn) error {
	const table = sqlclause.IPEdgesDailyTable
	const next = "traffic_edges_daily__next"
	createTable := func(name string) string {
		return ipEdgesCreateTableSQL(name, "day", "Date", "day", ipEdgesDailyTTL)
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
		needRebuild, reason, err := edgesDailyNeedsRebuild(ctx, ch, table)
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
			slog.Info("edges agg: traffic_edges_daily rebuilt", "reason", reason, "note", "backfill required")
		} else if err := ensureTTLOnlyDropPartsSetting(ctx, ch, table); err != nil {
			return err
		}
	}

	createMV := func(viewName string) string {
		return ipEdgesCreateMVSQL(viewName, table, "toDate(traffic_logs.timestamp)", "day")
	}
	if err := replaceMaterializedView(ctx, ch, sqlclause.IPEdgesDailyMV, createMV); err != nil {
		return err
	}
	return nil
}

// edgesDailyNeedsRebuild — true, если тип IP, зерно партиции или geo-колонки не совпадают с SoT.
func edgesDailyNeedsRebuild(ctx context.Context, ch clickhouse.Conn, table string) (bool, string, error) {
	typ, err := columnType(ctx, ch, table, "src_ip")
	if err != nil {
		return false, "", err
	}
	if !isIPv4Type(typ) {
		return true, "src_ip type " + typ, nil
	}
	pk, err := tablePartitionKey(ctx, ch, table)
	if err != nil {
		return false, "", err
	}
	if !isDayPartitionKey(pk) {
		return true, "partition_key " + pk, nil
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

func tablePartitionKey(ctx context.Context, ch clickhouse.Conn, table string) (string, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var pk string
	err := ch.QueryRow(qctx, `
		SELECT partition_key
		FROM system.tables
		WHERE database = currentDatabase() AND name = {table:String}
		LIMIT 1
	`, clickhouse.Named("table", table)).Scan(&pk)
	if err != nil {
		return "", fmt.Errorf("partition_key %s: %w", table, err)
	}
	return pk, nil
}

func isDayPartitionKey(pk string) bool {
	p := strings.TrimSpace(pk)
	return p == "day" || p == "`day`"
}

func ensureTTLOnlyDropPartsSetting(ctx context.Context, ch clickhouse.Conn, table string) error {
	ddl := fmt.Sprintf(`ALTER TABLE %s MODIFY SETTING ttl_only_drop_parts = 1`, table)
	if err := execDDL(ctx, ch, ddl); err != nil {
		return fmt.Errorf("ttl_only_drop_parts %s: %w", table, err)
	}
	return nil
}

// edgesAggReady — true, когда для каждого закрытого дня в traffic_logs есть агрегат.
func edgesAggReady(ctx context.Context, ch clickhouse.Conn) (bool, error) {
	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		return false, err
	}
	if rawRows == 0 {
		return true, nil
	}
	missing, err := missingClosedPartitionDays(ctx, ch, "traffic_logs", sqlclause.IPEdgesDailyTable)
	if err != nil {
		return false, err
	}
	return len(missing) == 0, nil
}

// BackfillEdgesAgg дозаполняет traffic_edges_daily по недостающим закрытым дням.
func BackfillEdgesAgg(ctx context.Context, ch clickhouse.Conn) error {
	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "error", Message: err.Error()})
		return fmt.Errorf("count traffic_logs: %w", err)
	}
	if rawRows == 0 {
		slog.Info("edges agg: traffic_logs empty, nothing to backfill")
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "ready", Message: "traffic_logs empty", RawRows: 0})
		return nil
	}

	aggRows, _ := countTableRows(ctx, ch, sqlclause.IPEdgesDailyTable)

	days, err := missingClosedPartitionDays(ctx, ch, "traffic_logs", sqlclause.IPEdgesDailyTable)
	if err != nil {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "error", Message: err.Error(), RawRows: rawRows, AggRows: aggRows})
		return err
	}
	if len(days) == 0 {
		slog.Info("edges agg: already up to date", "raw", rawRows, "agg", aggRows)
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "ready", Message: "up to date",
			RawRows: rawRows, AggRows: aggRows,
		})
		return nil
	}

	slog.Info("edges agg: backfill started", "raw", rawRows, "agg", aggRows, "days", len(days))
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "running", Phase: aggstate.PhaseBackfill, Message: "backfill in progress",
		RawRows: rawRows, AggRows: aggRows,
		DaysTotal: len(days), StartedAt: time.Now().UTC(),
	})

	if err := insertIPEdgesDays(ctx, ch, sqlclause.IPEdgesDailyTable, days, updateEdgesBackfillStatus(rawRows, len(days)), true); err != nil {
		return err
	}

	aggRows, _ = countTableRows(ctx, ch, sqlclause.IPEdgesDailyTable)
	slog.Info("edges agg: backfill complete", "days", len(days), "agg_rows", aggRows)
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "ready", Message: "backfill complete",
		RawRows: rawRows, AggRows: aggRows,
		DaysTotal: len(days), DaysDone: len(days),
		StartedAt: aggstate.GetEdgesAggStatus().StartedAt,
	})
	return nil
}

func updateEdgesBackfillStatus(rawRows uint64, total int) func(int) {
	started := time.Now().UTC()
	return func(done int) {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "running", Phase: aggstate.PhaseBackfill,
			Message: fmt.Sprintf("backfill %d/%d", done, total),
			RawRows: rawRows, DaysTotal: total, DaysDone: done,
			StartedAt: started,
		})
	}
}

func insertIPEdgesDays(ctx context.Context, ch clickhouse.Conn, table string, days []time.Time, onDay func(int), markError bool) error {
	if table != sqlclause.IPEdgesDailyTable && table != sqlclause.IPEdgesHourlyTable {
		return fmt.Errorf("insert IP edges: invalid table %q", table)
	}
	var (
		timeExpr, timeAlias, groupExtra string
	)
	switch table {
	case sqlclause.IPEdgesHourlyTable:
		timeExpr, timeAlias = "toStartOfHour(traffic_logs.timestamp)", "hour"
		groupExtra = "hour, src_ip, dst_ip"
	default:
		timeExpr, timeAlias = "toDate(traffic_logs.timestamp)", "day"
		groupExtra = "day, src_ip, dst_ip"
	}
	fromSQL := fmt.Sprintf("FROM traffic_logs\n\t\tWHERE %s", sqlclause.DayTimestampRangeSQL("traffic_logs.timestamp"))
	insertTpl := fmt.Sprintf(`
		INSERT INTO %s
		%s
		%s
		GROUP BY %s
		%s
	`, table, ipEdgesSelectBody(timeExpr, timeAlias), fromSQL, groupExtra, query.BackfillAggSettings())

	for i, day := range days {
		if err := ctx.Err(); err != nil {
			return err
		}
		ictx, icancel := context.WithTimeout(ctx, 30*time.Minute)
		err := ch.Exec(ictx, insertTpl, day, day)
		icancel()
		if err != nil {
			if markError {
				aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
					State: "error", Message: err.Error(), DaysTotal: len(days), DaysDone: i,
				})
			}
			return fmt.Errorf("backfill %s day %s: %w", table, day.Format("2006-01-02"), err)
		}
		slog.Info("ip edges: backfill day", "table", table, "done", i+1, "total", len(days), "day", day.Format("2006-01-02"))
		if onDay != nil {
			onDay(i + 1)
		}
	}
	return nil
}

func rebuildIPEdgesDays(ctx context.Context, ch clickhouse.Conn, table string, days []time.Time) error {
	if !isSafeTableIdent(table) {
		return fmt.Errorf("rebuild IP edges: invalid table %q", table)
	}
	exists, err := tableExists(ctx, ch, table)
	if err != nil || !exists {
		return err
	}
	for _, day := range days {
		if err := dropDatePartition(ctx, ch, table, day); err != nil {
			return err
		}
	}
	// markError=false: lookback rebuild must not flip UI to edges_agg_error.
	return insertIPEdgesDays(ctx, ch, table, days, nil, false)
}
