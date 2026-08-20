package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// countTableRows — оценка строк из system.parts (без full scan fact-таблицы).
func countTableRows(ctx context.Context, ch clickhouse.Conn, table string) (uint64, error) {
	if !isSafeTableIdent(table) {
		return 0, fmt.Errorf("count: invalid table %q", table)
	}
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	err := ch.QueryRow(qctx, `
		SELECT coalesce(sum(rows), 0)
		FROM system.parts
		WHERE database = currentDatabase() AND table = {table:String} AND active
	`, clickhouse.Named("table", table)).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func isSafeTableIdent(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

// missingClosedPartitionDays — календарные дни в raw, которых нет в agg, кроме today().
// Дни из system.parts (PARTITION BY toYYYYMMDD / Date), без DISTINCT по fact.
func missingClosedPartitionDays(ctx context.Context, ch clickhouse.Conn, rawTable, aggTable string) ([]time.Time, error) {
	return missingClosedPartitionDaysSince(ctx, ch, rawTable, aggTable, 0)
}

func missingClosedPartitionDaysSince(ctx context.Context, ch clickhouse.Conn, rawTable, aggTable string, lookbackDays int) ([]time.Time, error) {
	if !isSafeTableIdent(rawTable) || !isSafeTableIdent(aggTable) {
		return nil, fmt.Errorf("invalid table name")
	}
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	lookbackPred := ""
	if lookbackDays > 0 {
		lookbackPred = fmt.Sprintf("AND d >= today() - INTERVAL %d DAY", lookbackDays)
	}
	rows, err := ch.Query(qctx, fmt.Sprintf(`
		SELECT d
		FROM (
			SELECT DISTINCT %s AS d
			FROM system.parts
			WHERE database = currentDatabase() AND table = {raw:String} AND active
		) AS raw
		WHERE d < today() AND d > toDate('2000-01-01') %s
		AND d NOT IN (
			SELECT DISTINCT %s AS d
			FROM system.parts
			WHERE database = currentDatabase() AND table = {agg:String} AND active
		)
		ORDER BY d DESC
	`, partitionDayExpr(), lookbackPred, partitionDayExpr()),
		clickhouse.Named("raw", rawTable),
		clickhouse.Named("agg", aggTable),
	)
	if err != nil {
		return nil, fmt.Errorf("missing closed days %s vs %s: %w", rawTable, aggTable, err)
	}
	defer rows.Close()
	return scanDays(rows)
}

func partitionDayExpr() string {
	// partition — String: '20240814' (toYYYYMMDD) или '2024-08-14' (Date).
	return `toDate(parseDateTimeBestEffort(partition))`
}

func scanDays(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]time.Time, error) {
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

func dropDatePartition(ctx context.Context, ch clickhouse.Conn, table string, day time.Time) error {
	if !isSafeTableIdent(table) {
		return fmt.Errorf("drop partition: invalid table %q", table)
	}
	id, err := partitionIDForDay(ctx, ch, table, day)
	if err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	if strings.ContainsAny(id, "'\\\"/; ") {
		return fmt.Errorf("drop partition: unsafe partition_id %q", id)
	}
	ddl := fmt.Sprintf("ALTER TABLE %s DROP PARTITION ID '%s'", table, id)
	dctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if err := ch.Exec(dctx, ddl); err != nil {
		return fmt.Errorf("drop partition %s %s: %w", table, day.Format("2006-01-02"), err)
	}
	return nil
}

func partitionIDForDay(ctx context.Context, ch clickhouse.Conn, table string, day time.Time) (string, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var id string
	// clickhouse-go binds time.Time for {day:Date} as Unix seconds; CH Date expects YYYY-MM-DD.
	err := ch.QueryRow(qctx, `
		SELECT any(partition_id)
		FROM system.parts
		WHERE database = currentDatabase() AND table = {table:String} AND active
		  AND toDate(parseDateTimeBestEffort(partition)) = {day:Date}
	`, clickhouse.Named("table", table), clickhouse.Named("day", dateParam(day))).Scan(&id)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "no rows") || strings.Contains(msg, "empty result") {
			return "", nil
		}
		return "", err
	}
	return id, nil
}

// dateParam formats a calendar day for ClickHouse {name:Date} query parameters.
func dateParam(day time.Time) string {
	return day.UTC().Format("2006-01-02")
}

func normalizeSortingKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

func tableSortingKey(ctx context.Context, ch clickhouse.Conn, table string) (string, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var key string
	err := ch.QueryRow(qctx, `
		SELECT sorting_key
		FROM system.tables
		WHERE database = currentDatabase() AND name = {table:String}
		LIMIT 1
	`, clickhouse.Named("table", table)).Scan(&key)
	if err != nil {
		return "", fmt.Errorf("sorting_key %s: %w", table, err)
	}
	return key, nil
}

func columnExists(ctx context.Context, ch clickhouse.Conn, table, column string) (bool, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	err := ch.QueryRow(qctx, `
		SELECT count()
		FROM system.columns
		WHERE database = currentDatabase() AND table = {table:String} AND name = {column:String}
	`, clickhouse.Named("table", table), clickhouse.Named("column", column)).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
