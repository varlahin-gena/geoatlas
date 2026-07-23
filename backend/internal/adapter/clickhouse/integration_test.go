//go:build integration

package clickhouse_test

import (
	"context"
	"os"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"

	"network_monitor/internal/adapter/clickhouse"
	"network_monitor/internal/adapter/clickhouse/aggstate"
	"network_monitor/internal/adapter/clickhouse/query"
	"network_monitor/internal/model"
)

func testCH(t *testing.T) ch.Conn {
	t.Helper()
	addr := os.Getenv("CLICKHOUSE_TEST_ADDR")
	if addr == "" {
		t.Skip("CLICKHOUSE_TEST_ADDR not set")
	}
	user := os.Getenv("CLICKHOUSE_TEST_USER")
	if user == "" {
		user = "default"
	}
	db := os.Getenv("CLICKHOUSE_TEST_DB")
	if db == "" {
		db = "default"
	}
	auth := clickhouse.Auth{
		Username: user,
		Password: os.Getenv("CLICKHOUSE_TEST_PASSWORD"),
		Database: db,
	}
	// GHA service: HTTP /ping может подняться раньше native auth — даём запас на retry.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	conn, err := clickhouse.ConnectWithPool(ctx, addr, auth, clickhouse.PoolOptions{
		Name:         "test",
		MaxOpenConns: 2,
		MaxIdleConns: 2,
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var version string
	if err := conn.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		t.Fatalf("read ClickHouse version: %v", err)
	}
	t.Logf("ClickHouse version: %s", version)
	return conn
}

func ensureSchema(t *testing.T, conn ch.Conn) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS traffic_logs (
			timestamp DateTime64(3),
			parsed_at DateTime64(3) DEFAULT now64(3),
			vendor LowCardinality(String) DEFAULT '',
			device String,
			src_ip String, dst_ip String,
			src_port UInt32, dst_port UInt32,
			action LowCardinality(String),
			rule String, proto LowCardinality(String),
			src_zone String, dst_zone String,
			src_country String, dst_country String,
			src_city String DEFAULT '', dst_city String DEFAULT '',
			src_region String DEFAULT '', dst_region String DEFAULT '',
			src_lat Float64 DEFAULT 0, src_lon Float64 DEFAULT 0,
			dst_lat Float64 DEFAULT 0, dst_lon Float64 DEFAULT 0,
			bytes_sent UInt64, bytes_recv UInt64,
			packets_sent UInt64, packets_recv UInt64,
			raw String
		) ENGINE = MergeTree() ORDER BY (timestamp, src_ip, dst_ip)`,
		`CREATE TABLE IF NOT EXISTS parse_errors (
			id UUID DEFAULT generateUUIDv4(),
			timestamp DateTime64(3) DEFAULT now64(3),
			vendor LowCardinality(String) DEFAULT '',
			reason String,
			raw String
		) ENGINE = MergeTree() ORDER BY timestamp`,
		`TRUNCATE TABLE traffic_logs`,
		`TRUNCATE TABLE parse_errors`,
	}
	for _, s := range stmts {
		if err := conn.Exec(ctx, s); err != nil {
			t.Fatalf("schema: %v\nSQL: %s", err, s)
		}
	}
}

func TestIntegrationInsertAndScan(t *testing.T) {
	conn := testCH(t)
	ensureSchema(t, conn)
	// ScanRawAggs читает edges_daily только при ready — в CI таблицы нет.
	aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "integration test"})
	t.Cleanup(func() {
		aggstate.SetEdgesAggStatus(aggstate.EdgesAggStatus{State: "idle", Message: "not started"})
	})
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	logs := []model.TrafficLog{{
		Timestamp:  now,
		ParsedAt:   now,
		Vendor:     "generic",
		Device:     "fw01",
		SrcIP:      "10.0.0.1",
		DstIP:      "8.8.8.8",
		SrcPort:    12345,
		DstPort:    443,
		Action:     "allow",
		Proto:      "tcp",
		SrcCountry: "",
		DstCountry: "United States",
		Raw:        "test line",
	}, {
		Timestamp:  now,
		ParsedAt:   now,
		Vendor:     "generic",
		Device:     "fw01",
		SrcIP:      "10.0.0.1",
		DstIP:      "8.8.8.8",
		SrcPort:    12345,
		DstPort:    443,
		Action:     "deny",
		Proto:      "tcp",
		SrcCountry: "",
		DstCountry: "United States",
		Raw:        "blocked test line",
	}}
	if err := clickhouse.InsertTrafficLogs(ctx, conn, logs); err != nil {
		t.Fatalf("InsertTrafficLogs: %v", err)
	}

	raws, err := query.ScanRawAggs(ctx, conn, 1, 100, "all", time.Minute)
	if err != nil {
		t.Fatalf("ScanRawAggs: %v", err)
	}
	if len(raws) == 0 {
		t.Fatal("ScanRawAggs: no rows")
	}
	found := false
	for _, r := range raws {
		if r.SrcIP == "10.0.0.1" && r.DstIP == "8.8.8.8" {
			if r.Count != 2 || r.AllowedCnt != 1 || r.BlockedCnt != 1 {
				t.Fatalf("aggregate counts = count:%d allowed:%d blocked:%d, want 2/1/1",
					r.Count, r.AllowedCnt, r.BlockedCnt)
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected pair 10.0.0.1→8.8.8.8 in %+v", raws)
	}
}

func TestIntegrationInsertParseErrors(t *testing.T) {
	conn := testCH(t)
	ensureSchema(t, conn)
	ctx := context.Background()

	err := clickhouse.InsertParseErrors(ctx, conn, []model.ParseError{{
		Timestamp: time.Now().UTC(),
		Vendor:    "test",
		Reason:    "boom",
		Raw:       "garbage line",
	}})
	if err != nil {
		t.Fatalf("InsertParseErrors: %v", err)
	}
	rows, err := clickhouse.ListParseErrors(ctx, conn, 10, "garbage")
	if err != nil {
		t.Fatalf("ListParseErrors: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("count = %d, want 1", len(rows))
	}
	if rows[0].Reason != "boom" {
		t.Fatalf("reason = %q", rows[0].Reason)
	}
}
