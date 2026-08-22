package migrate

import (
	"strings"
	"testing"
	"time"

	"network_monitor/internal/adapter/clickhouse/sqlclause"
)

func TestNormalizeSortingKey(t *testing.T) {
	got := normalizeSortingKey("toStartOfHour(timestamp), src_ip, dst_ip")
	if got != trafficLogsDesiredOrder {
		t.Fatalf("got %q want %q", got, trafficLogsDesiredOrder)
	}
	if normalizeSortingKey("timestamp, src_ip, dst_ip, action") == trafficLogsDesiredOrder {
		t.Fatal("old key must not match")
	}
}

func TestIsSafeTableIdent(t *testing.T) {
	if !isSafeTableIdent("traffic_logs") || !isSafeTableIdent("traffic_edges_daily") {
		t.Fatal("valid idents")
	}
	for _, bad := range []string{"", "a; DROP", "x y", "t-1", "traffic_logs`"} {
		if isSafeTableIdent(bad) {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestIPEdgesCreateSQLHasCoords(t *testing.T) {
	ddl := ipEdgesCreateTableSQL("traffic_edges_daily", "day", "Date", "day", ipEdgesDailyTTL)
	for _, want := range []string{"coord_weight", "src_lat_sum", "src_city", "PARTITION BY day", "ttl_only_drop_parts"} {
		if !strings.Contains(ddl, want) {
			t.Fatalf("missing %q in:\n%s", want, ddl)
		}
	}
	mv := ipEdgesCreateMVSQL(sqlclause.IPEdgesDailyMV, sqlclause.IPEdgesDailyTable, "toDate(traffic_logs.timestamp)", "day")
	for _, want := range []string{"coord_weight", "anyState(traffic_logs.src_city)", "GROUP BY day, src_ip, dst_ip"} {
		if !strings.Contains(mv, want) {
			t.Fatalf("missing %q in:\n%s", want, mv)
		}
	}
}

func TestTrafficLogsCreateSQLLayout(t *testing.T) {
	ddl := trafficLogsIPv4CreateSQL("traffic_logs")
	if strings.Contains(ddl, "raw") {
		t.Fatal("raw column must be dropped")
	}
	if strings.Contains(ddl, "IF NOT EXISTS") {
		t.Fatal("rebuild/EXCHANGE CREATE must not use IF NOT EXISTS")
	}
	if !strings.Contains(ddl, "ORDER BY (toStartOfHour(timestamp), src_ip, dst_ip)") {
		t.Fatalf("unexpected ORDER BY:\n%s", ddl)
	}
	if !strings.Contains(ddl, "LowCardinality(String)") || !strings.Contains(ddl, "src_country") {
		t.Fatal("expected LC geo columns")
	}
	ifNot := trafficLogsCreateSQL("traffic_logs", true)
	if !strings.Contains(ifNot, "CREATE TABLE IF NOT EXISTS traffic_logs") {
		t.Fatalf("bootstrap CREATE must be IF NOT EXISTS:\n%s", ifNot)
	}
}

func TestDayTimestampRangeSQL(t *testing.T) {
	got := sqlclause.DayTimestampRangeSQL("traffic_logs.timestamp")
	if !strings.Contains(got, "toDateTime(?)") || !strings.Contains(got, "INTERVAL 1 DAY") {
		t.Fatal(got)
	}
}

func TestHourTimestampRangeSQL(t *testing.T) {
	got := sqlclause.HourTimestampRangeSQL("traffic_logs.timestamp")
	if !strings.Contains(got, "toDateTime(?)") || !strings.Contains(got, "INTERVAL 1 HOUR") {
		t.Fatal(got)
	}
}

func TestDateParam(t *testing.T) {
	day := time.Date(2026, 8, 20, 15, 30, 0, 0, time.FixedZone("MSK", 3*3600))
	got := dateParam(day)
	if got != "2026-08-20" {
		t.Fatalf("dateParam=%q want 2026-08-20", got)
	}
}
