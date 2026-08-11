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

func TestRestoreTableAsSQL(t *testing.T) {
	got := restoreTableAsSQL("traffic_logs", "nm_bak_traffic_logs", "nm-20260101T000000Z")
	want := "RESTORE TABLE traffic_logs AS nm_bak_traffic_logs FROM Disk('backups', 'nm-20260101T000000Z')"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
