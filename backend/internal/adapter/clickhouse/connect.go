package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// Connect — совместимая обёртка над ConnectWithPool с дефолтным размером пула.
func Connect(ctx context.Context, addr string) (clickhouse.Conn, error) {
	return ConnectWithPool(ctx, addr, Auth{}, PoolOptions{Name: "default"})
}
