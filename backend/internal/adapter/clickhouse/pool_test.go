package clickhouse

import (
	"context"
	"testing"
	"time"
)

func TestConnectWithPoolFailsFast(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := ConnectWithPool(ctx, "127.0.0.1:1", Auth{}, PoolOptions{Name: "t", MaxOpenConns: 1, MaxIdleConns: 1})
	if err == nil {
		t.Fatal("expected connect error")
	}
	_, err = ConnectPools(ctx, "127.0.0.1:1", Auth{}, PoolOptions{MaxOpenConns: 1}, PoolOptions{MaxOpenConns: 1}, PoolOptions{MaxOpenConns: 1})
	if err == nil {
		t.Fatal("expected pools error")
	}
}
