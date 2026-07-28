package migrate

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// replaceMaterializedView создаёт новую MV под именем name_next, затем атомарно
// меняет её местами с name через EXCHANGE TABLES. Пока CREATE идёт, старая MV
// продолжает писать — нет окна «без агрегации» как при DROP+CREATE.
//
// createSQL(viewName) должен вернуть полный CREATE MATERIALIZED VIEW <viewName> ...
func replaceMaterializedView(ctx context.Context, ch clickhouse.Conn, name string, createSQL func(viewName string) string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("materialized view name is empty")
	}
	if createSQL == nil {
		return fmt.Errorf("createSQL is nil")
	}

	exists, err := tableExists(ctx, ch, name)
	if err != nil {
		return err
	}
	if !exists {
		if err := execDDL(ctx, ch, createSQL(name)); err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
		return nil
	}

	next := name + "_next"
	_ = execDDL(ctx, ch, fmt.Sprintf("DROP TABLE IF EXISTS %s", next))
	if err := execDDL(ctx, ch, createSQL(next)); err != nil {
		_ = execDDL(ctx, ch, fmt.Sprintf("DROP TABLE IF EXISTS %s", next))
		return fmt.Errorf("create %s: %w", next, err)
	}
	if err := execDDL(ctx, ch, fmt.Sprintf("EXCHANGE TABLES %s AND %s", name, next)); err != nil {
		_ = execDDL(ctx, ch, fmt.Sprintf("DROP TABLE IF EXISTS %s", next))
		return fmt.Errorf("exchange %s <-> %s: %w", name, next, err)
	}
	// После EXCHANGE в next лежит прежнее определение — убираем.
	if err := execDDL(ctx, ch, fmt.Sprintf("DROP TABLE IF EXISTS %s", next)); err != nil {
		return fmt.Errorf("drop old %s: %w", next, err)
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
		WHERE database = currentDatabase() AND name = {name:String}
	`, clickhouse.Named("name", name)).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("tableExists %s: %w", name, err)
	}
	return n > 0, nil
}
