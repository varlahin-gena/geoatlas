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

// Схема версий: при изменении DDL MV/колонок бампим константу —
// Ensure* пересоздаст объекты, если stored < desired.
const (
	schemaComponentEdgesAgg = "edges_agg"
	schemaComponentGeoEdges = "geo_edges_agg"

	schemaVersionEdgesAgg uint32 = 1
	schemaVersionGeoEdges uint32 = 1
)

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
			// Компонент ещё не зарегистрирован — версия 0.
			return 0, nil
		}
		// Реальная ошибка CH/сети: не притворяемся, что DDL нужен.
		return 0, fmt.Errorf("read schema version %q: %w", component, err)
	}
	return v, nil
}

// isNoSchemaVersionRow — true только для «строки нет», не для timeout/сети.
func isNoSchemaVersionRow(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return true
	}
	// clickhouse-go иногда оборачивает отсутствие строк без sql.ErrNoRows.
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

// needsSchemaDDLFn — injectable для тестов (DROP MV не должен вызываться при ошибке).
var needsSchemaDDLFn = needsSchemaDDL

// needsSchemaDDL — true, если stored < desired (нужно применить CREATE/ALTER).
// При ошибке чтения версии возвращает (false, err): вызывающий НЕ должен DROP MV.
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
