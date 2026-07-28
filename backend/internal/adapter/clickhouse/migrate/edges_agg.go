package migrate

import (
	"context"
	"fmt"
	"log/slog"
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

// EnsureEdgesAgg — schema + backfill (полный путь; startup по умолчанию / ops-скрипты).
func EnsureEdgesAgg(ctx context.Context, ch clickhouse.Conn) error {
	if err := EnsureEdgesAggSchema(ctx, ch); err != nil {
		return err
	}
	return BackfillEdgesAgg(ctx, ch)
}

// RefreshEdgesAggReady помечает агрегаты ready без INSERT, если дни уже догнаны.
// Иначе оставляет State=pending (карта читает traffic_logs).
func RefreshEdgesAggReady(ctx context.Context, ch clickhouse.Conn) error {
	rawRows, err := countTableRows(ctx, ch, "traffic_logs")
	if err != nil {
		return err
	}
	aggRows, _ := countTableRows(ctx, ch, "traffic_edges_daily")
	ready, err := edgesAggReady(ctx, ch)
	if err != nil {
		return err
	}
	if ready {
		msg := "up to date"
		if rawRows == 0 {
			msg = "traffic_logs empty"
		}
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "ready", Message: msg, RawRows: rawRows, AggRows: aggRows,
		})
		return nil
	}
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "pending", Message: "backfill needed (SKIP_STARTUP_BACKFILL or incomplete)",
		RawRows: rawRows, AggRows: aggRows,
	})
	return nil
}

func applyEdgesAggSchema(ctx context.Context, ch clickhouse.Conn) error {
	// Таблицу создаём IF NOT EXISTS; MV — через create→EXCHANGE, без окна DROP.
	if err := execDDL(ctx, ch, `
		CREATE TABLE IF NOT EXISTS traffic_edges_daily
		(
			day           Date,
			src_ip        String,
			dst_ip        String,
			cnt           SimpleAggregateFunction(sum, UInt64),
			blocked_cnt   SimpleAggregateFunction(sum, UInt64),
			allowed_cnt   SimpleAggregateFunction(sum, UInt64),
			bytes_sent    SimpleAggregateFunction(sum, UInt64),
			bytes_recv    SimpleAggregateFunction(sum, UInt64),
			packets_sent  SimpleAggregateFunction(sum, UInt64),
			packets_recv  SimpleAggregateFunction(sum, UInt64),
			last_action   AggregateFunction(argMax, String, DateTime64(3)),
			rule          AggregateFunction(any, String),
			proto         AggregateFunction(any, String),
			src_port      AggregateFunction(any, UInt32),
			dst_port      AggregateFunction(any, UInt32),
			device        AggregateFunction(any, String),
			src_zone      AggregateFunction(any, String),
			dst_zone      AggregateFunction(any, String),
			src_country   AggregateFunction(any, String),
			dst_country   AggregateFunction(any, String)
		)
		ENGINE = AggregatingMergeTree()
		PARTITION BY toYYYYMM(day)
		ORDER BY (day, src_ip, dst_ip)
		TTL day + INTERVAL 30 DAY DELETE
	`); err != nil {
		return fmt.Errorf("create traffic_edges_daily: %w", err)
	}

	createMV := func(viewName string) string {
		return fmt.Sprintf(`
		CREATE MATERIALIZED VIEW %s
		TO traffic_edges_daily AS
		SELECT
			toDate(timestamp) AS day,
			src_ip,
			dst_ip,
			count() AS cnt,
			%s AS blocked_cnt,
			%s AS allowed_cnt,
			sum(bytes_sent) AS bytes_sent,
			sum(bytes_recv) AS bytes_recv,
			sum(packets_sent) AS packets_sent,
			sum(packets_recv) AS packets_recv,
			argMaxState(action, timestamp) AS last_action,
			anyState(rule)          AS rule,
			anyState(proto)         AS proto,
			anyState(src_port)      AS src_port,
			anyState(dst_port)      AS dst_port,
			anyState(device)        AS device,
			anyState(src_zone)      AS src_zone,
			anyState(dst_zone)      AS dst_zone,
			anyState(src_country)   AS src_country,
			anyState(dst_country)   AS dst_country
		FROM traffic_logs
		GROUP BY day, src_ip, dst_ip
	`, viewName, sqlclause.SumBlockedSQL(), sqlclause.SumAllowedSQL())
	}
	if err := replaceMaterializedView(ctx, ch, "traffic_edges_daily_mv", createMV); err != nil {
		return err
	}
	return nil
}

// edgesAggReady — true, когда для каждого дня в traffic_logs есть агрегат.
func edgesAggReady(ctx context.Context, ch clickhouse.Conn) (bool, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var rawRows uint64
	if err := ch.QueryRow(qctx, `SELECT count() FROM traffic_logs`).Scan(&rawRows); err != nil {
		return false, err
	}
	if rawRows == 0 {
		return true, nil
	}

	var missing uint64
	err := ch.QueryRow(qctx, `
		SELECT count()
		FROM (
			SELECT DISTINCT toDate(timestamp) AS d FROM traffic_logs
		) AS days
		LEFT ANTI JOIN (
			SELECT DISTINCT day AS d FROM traffic_edges_daily
		) AS agg USING (d)
	`).Scan(&missing)
	if err != nil {
		return false, err
	}
	return missing == 0, nil
}

// BackfillEdgesAgg дозаполняет traffic_edges_daily по недостающим дням.
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

	aggRows, _ := countTableRows(ctx, ch, "traffic_edges_daily")

	ready, err := edgesAggReady(ctx, ch)
	if err != nil {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "error", Message: err.Error(), RawRows: rawRows, AggRows: aggRows})
		return err
	}
	if ready {
		slog.Info("edges agg: already up to date", "raw", rawRows, "agg", aggRows)
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "ready", Message: "up to date",
			RawRows: rawRows, AggRows: aggRows,
		})
		return nil
	}

	slog.Info("edges agg: backfill started", "raw", rawRows, "agg", aggRows)
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "running", Phase: aggstate.PhaseBackfill, Message: "backfill started",
		RawRows: rawRows, AggRows: aggRows, StartedAt: time.Now().UTC(),
	})

	// Список дней, для которых ещё нет агрегатов.
	dctx, dcancel := context.WithTimeout(ctx, 2*time.Minute)
	defer dcancel()

	rows, err := ch.Query(dctx, `
		SELECT days.d AS day
		FROM (
			SELECT DISTINCT toDate(timestamp) AS d FROM traffic_logs
		) AS days
		LEFT ANTI JOIN (
			SELECT DISTINCT day AS d FROM traffic_edges_daily
		) AS agg USING (d)
		ORDER BY day DESC
	`)
	if err != nil {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "error", Message: err.Error(), RawRows: rawRows, AggRows: aggRows})
		return fmt.Errorf("list days for backfill: %w", err)
	}
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var day time.Time
		if err := rows.Scan(&day); err != nil {
			return err
		}
		days = append(days, day)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(days) == 0 {
		slog.Warn("edges agg: missing days reported but list is empty", "raw", rawRows)
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "error", Message: "missing days list empty",
			RawRows: rawRows, AggRows: aggRows,
		})
		return fmt.Errorf("edges agg: inconsistent state: backfill needed but no days found")
	}

	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "running", Phase: aggstate.PhaseBackfill, Message: "backfill in progress",
		RawRows: rawRows, AggRows: aggRows,
		DaysTotal: len(days), StartedAt: time.Now().UTC(),
	})

	insertTpl := fmt.Sprintf(`
		INSERT INTO traffic_edges_daily
		SELECT
			toDate(timestamp) AS day,
			src_ip,
			dst_ip,
			count() AS cnt,
			%s AS blocked_cnt,
			%s AS allowed_cnt,
			sum(bytes_sent) AS bytes_sent,
			sum(bytes_recv) AS bytes_recv,
			sum(packets_sent) AS packets_sent,
			sum(packets_recv) AS packets_recv,
			argMaxState(action, timestamp) AS last_action,
			anyState(rule)          AS rule,
			anyState(proto)         AS proto,
			anyState(src_port)      AS src_port,
			anyState(dst_port)      AS dst_port,
			anyState(device)        AS device,
			anyState(src_zone)      AS src_zone,
			anyState(dst_zone)      AS dst_zone,
			anyState(src_country)   AS src_country,
			anyState(dst_country)   AS dst_country
		FROM traffic_logs
		WHERE toDate(timestamp) = ?
		GROUP BY day, src_ip, dst_ip
		%s
	`, sqlclause.SumBlockedSQL(), sqlclause.SumAllowedSQL(), query.AggSettings())

	for i, day := range days {
		if err := ctx.Err(); err != nil {
			return err
		}
		ictx, icancel := context.WithTimeout(ctx, 30*time.Minute)
		err := ch.Exec(ictx, insertTpl, day)
		icancel()
		if err != nil {
			aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
				State: "error", Message: err.Error(),
				RawRows: rawRows, DaysTotal: len(days), DaysDone: i,
			})
			return fmt.Errorf("backfill day %s: %w", day.Format("2006-01-02"), err)
		}
		slog.Info("edges agg: backfill day", "done", i+1, "total", len(days), "day", day.Format("2006-01-02"))
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
			State: "running", Phase: aggstate.PhaseBackfill,
			Message: fmt.Sprintf("backfill %d/%d", i+1, len(days)),
			RawRows: rawRows, DaysTotal: len(days), DaysDone: i + 1,
			StartedAt: aggstate.GetEdgesAggStatus().StartedAt,
		})
	}

	aggRows, _ = countTableRows(ctx, ch, "traffic_edges_daily")
	slog.Info("edges agg: backfill complete", "days", len(days), "agg_rows", aggRows)
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{
		State: "ready", Message: "backfill complete",
		RawRows: rawRows, AggRows: aggRows,
		DaysTotal: len(days), DaysDone: len(days),
		StartedAt: aggstate.GetEdgesAggStatus().StartedAt,
	})
	return nil
}
