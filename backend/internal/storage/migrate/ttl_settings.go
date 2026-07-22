package migrate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const (
	schemaComponentTTLDropParts = "ttl_only_drop_parts"
	schemaVersionTTLDropParts   uint32 = 1
)

// EnsureTTLOnlyDropParts включает ttl_only_drop_parts на таблицах с дневными
// партициями: истёкшие данные удаляются drop'ом партиции, без row-level TTL merges.
func EnsureTTLOnlyDropParts(ctx context.Context, ch clickhouse.Conn) error {
	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentTTLDropParts, schemaVersionTTLDropParts)
	if err != nil {
		return err
	}
	if !needDDL {
		return nil
	}

	tables := []string{"traffic_logs", "parse_errors", "system_metrics"}
	for _, table := range tables {
		ok, err := tableExists(ctx, ch, table)
		if err != nil {
			return fmt.Errorf("check table %s: %w", table, err)
		}
		if !ok {
			slog.Info("ttl: skip missing table", "table", table)
			continue
		}
		ddl := fmt.Sprintf(`ALTER TABLE %s MODIFY SETTING ttl_only_drop_parts = 1`, table)
		if err := execDDL(ctx, ch, ddl); err != nil {
			return fmt.Errorf("ttl_only_drop_parts %s: %w", table, err)
		}
		slog.Info("ttl: ttl_only_drop_parts enabled", "table", table)
	}

	if err := setSchemaVersion(ctx, ch, schemaComponentTTLDropParts, schemaVersionTTLDropParts); err != nil {
		return fmt.Errorf("set ttl_only_drop_parts schema version: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, ch clickhouse.Conn, name string) (bool, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	err := ch.QueryRow(qctx, `
		SELECT count()
		FROM system.tables
		WHERE database = currentDatabase() AND name = ?
	`, name).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
