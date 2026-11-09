package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const (
	schemaComponentConnectorID        = "traffic_logs_connector_id"
	schemaVersionConnectorID   uint32 = 1
)

// EnsureTrafficLogsConnectorID adds connector_id for ops multi-port diagnostics.
func EnsureTrafficLogsConnectorID(ctx context.Context, ch clickhouse.Conn) error {
	needDDL, err := needsSchemaDDLFn(ctx, ch, schemaComponentConnectorID, schemaVersionConnectorID)
	if err != nil {
		return err
	}
	if !needDDL {
		return nil
	}
	slog.Info("traffic_logs: adding connector_id column")
	ddl := `ALTER TABLE traffic_logs ADD COLUMN IF NOT EXISTS connector_id LowCardinality(String) DEFAULT ''`
	if err := execDDL(ctx, ch, ddl); err != nil {
		return fmt.Errorf("add connector_id: %w", err)
	}
	if err := setSchemaVersion(ctx, ch, schemaComponentConnectorID, schemaVersionConnectorID); err != nil {
		return fmt.Errorf("set connector_id schema version: %w", err)
	}
	slog.Info("traffic_logs: connector_id ready", "version", schemaVersionConnectorID)
	return nil
}
