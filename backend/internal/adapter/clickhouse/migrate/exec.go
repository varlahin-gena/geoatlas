package migrate

import (
	"context"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func execDDL(ctx context.Context, ch clickhouse.Conn, query string) error {
	qctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return ch.Exec(qctx, query)
}
