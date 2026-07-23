package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func execDDL(ctx context.Context, ch clickhouse.Conn, query string) error {
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return ch.Exec(qctx, query)
}

func countTableRows(ctx context.Context, ch clickhouse.Conn, table string) (uint64, error) {
	qctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var n uint64
	if err := ch.QueryRow(qctx, fmt.Sprintf("SELECT count() FROM %s", table)).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
