package clickhouse

import (
	"strings"
	"testing"
)

func TestBackupTablesSQLPrefixesEachTable(t *testing.T) {
	got := backupTablesSQL([]string{"traffic_logs", "geo_ranges"}, "nm-20260101T000000Z")
	want := "BACKUP TABLE traffic_logs, TABLE geo_ranges TO Disk('backups', 'nm-20260101T000000Z')"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if strings.Contains(got, "TABLE traffic_logs, geo_ranges") {
		t.Fatal("must not omit TABLE before subsequent names")
	}
}
