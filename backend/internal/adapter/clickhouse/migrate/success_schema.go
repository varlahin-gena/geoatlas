package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const (
	schemaComponentTrafficSuccess        = "traffic_logs_success"
	schemaVersionTrafficSuccess   uint32 = 1
)

// EnsureTrafficLogsSuccess обновляет выражение MATERIALIZED колонки success
// до model.AllowedInClause() (UserGate/Forti/Cisco allow-синонимы).
// Для уже существующих parts значения не пересчитываются; новые INSERT — по новой формуле.
func EnsureTrafficLogsSuccess(ctx context.Context, ch clickhouse.Conn) error {
	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentTrafficSuccess, schemaVersionTrafficSuccess)
	if err != nil {
		return err
	}
	if !needDDL {
		return nil
	}
	slog.Info("traffic_logs: updating success MATERIALIZED expression")
	ddl := fmt.Sprintf(`
		ALTER TABLE traffic_logs
		MODIFY COLUMN success UInt8 MATERIALIZED
			%s
	`, trafficLogsSuccessExpr())
	if err := execDDL(ctx, ch, ddl); err != nil {
		return fmt.Errorf("modify success column: %w", err)
	}
	if err := setSchemaVersion(ctx, ch, schemaComponentTrafficSuccess, schemaVersionTrafficSuccess); err != nil {
		return fmt.Errorf("set traffic_logs_success schema version: %w", err)
	}
	slog.Info("traffic_logs: success expression updated", "version", schemaVersionTrafficSuccess)
	return nil
}
