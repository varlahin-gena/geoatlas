package ingeststore

import (
	"reflect"
	"testing"
	"time"

	"geoatlas/internal/model"
)

func TestPackTrafficColumnsShape(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	logs := []model.TrafficLog{
		{Vendor: "fortigate", SrcIP: "10.0.0.1", DstIP: "8.8.8.8", SrcPort: 1, DstPort: 443, Action: "allow"},
		{Vendor: "usergate", ParsedAt: now.Add(-time.Second), SrcIP: "10.0.0.2", DstIP: "1.1.1.1"},
	}
	cols := packTrafficColumns(logs, now)
	if len(cols) != 27 {
		t.Fatalf("columns = %d, want 27 (INSERT list)", len(cols))
	}
	for i, col := range cols {
		n := reflect.ValueOf(col).Len()
		if n != len(logs) {
			t.Fatalf("col %d len = %d, want %d", i, n, len(logs))
		}
	}
	vendors := cols[2].([]string)
	if vendors[0] != "fortigate" || vendors[1] != "usergate" {
		t.Fatalf("vendors = %v", vendors)
	}
	parsed := cols[1].([]time.Time)
	if !parsed[0].Equal(now) {
		t.Fatalf("zero ParsedAt should use now, got %v", parsed[0])
	}
	if !parsed[1].Equal(now.Add(-time.Second)) {
		t.Fatalf("explicit ParsedAt overwritten: %v", parsed[1])
	}
}

func BenchmarkPackTrafficColumns(b *testing.B) {
	const batch = 10000
	logs := make([]model.TrafficLog, batch)
	now := time.Now()
	for i := range logs {
		logs[i] = model.TrafficLog{
			Timestamp: now,
			ParsedAt:  now,
			Vendor:    "fortigate",
			SrcIP:     "10.1.100.11",
			DstIP:     "52.53.140.235",
			SrcPort:   54190,
			DstPort:   443,
			Action:    "close",
			Proto:     "tcp",
			BytesSent: 3652,
			BytesRecv: 146668,
		}
	}
	b.ReportAllocs()
	b.SetBytes(int64(batch))
	for b.Loop() {
		_ = packTrafficColumns(logs, now)
	}
}
