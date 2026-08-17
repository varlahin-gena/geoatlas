package migrate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"network_monitor/internal/model"
)

func TestGeneratedSchemaArtifactsUpToDate(t *testing.T) {
	root := schemaRepoRoot(t)
	cases := []struct {
		rel  string
		body string
	}{
		{filepath.Join("clickhouse", "init.sql"), RenderColdBootstrapSQL()},
		{filepath.Join("clickhouse", "migrate_success.sql"), RenderMigrateSuccessSQL()},
		{filepath.Join("clickhouse", "migrate_edges_agg.sql"), RenderMigrateEdgesAggSQL()},
	}
	for _, c := range cases {
		path := filepath.Join(root, c.rel)
		raw, err := os.ReadFile(path) //nolint:gosec // test fixture under repo root
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		got := strings.ReplaceAll(string(raw), "\r\n", "\n")
		want := strings.ReplaceAll(c.body, "\r\n", "\n")
		if got != want {
			t.Errorf("%s is out of date; run: cd backend && go generate ./internal/adapter/clickhouse/migrate/...", c.rel)
		}
	}
}

func TestColdBootstrapSQLHasNoEdgesAgg(t *testing.T) {
	raw := RenderColdBootstrapSQL()
	if strings.Contains(raw, "traffic_edges_daily_mv") {
		t.Error("init bootstrap must not contain traffic_edges_daily_mv")
	}
	if strings.Contains(raw, "CREATE MATERIALIZED VIEW") {
		t.Error("init bootstrap must not create materialized views")
	}
	if !strings.Contains(raw, "CREATE TABLE IF NOT EXISTS traffic_logs") {
		t.Fatal("missing traffic_logs")
	}
	if !strings.Contains(raw, "ttl_only_drop_parts = 1;") {
		t.Fatal("bootstrap statements must end with semicolon for clickhouse-client --multiquery")
	}
	if !strings.Contains(raw, model.AllowedInClause()) {
		t.Fatal("traffic_logs success expr must use AllowedInClause")
	}
}

func TestOpsFallbackSQLMatchesRuntimeSoT(t *testing.T) {
	success := RenderMigrateSuccessSQL()
	if !strings.Contains(success, trafficLogsSuccessExpr()) {
		t.Fatal("migrate_success.sql must use trafficLogsSuccessExpr")
	}
	edges := RenderMigrateEdgesAggSQL()
	if !strings.Contains(edges, "CREATE TABLE IF NOT EXISTS traffic_edges_daily") {
		t.Fatal("missing IF NOT EXISTS traffic_edges_daily")
	}
	if !strings.Contains(edges, "CREATE MATERIALIZED VIEW IF NOT EXISTS traffic_edges_daily_mv") {
		t.Fatal("missing IF NOT EXISTS MV")
	}
	if !strings.Contains(edges, "ttl_only_drop_parts = 1;") {
		t.Fatal("ops fallback statements must end with semicolon")
	}
	if !strings.Contains(edges, model.BlockedInClause()) {
		t.Fatal("edges MV must use BlockedInClause")
	}
}

func TestEnsureBaseSchemaNilConn(t *testing.T) {
	if err := EnsureBaseSchema(context.Background(), nil); err == nil {
		t.Fatal("expected error")
	}
}

func schemaRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "clickhouse")); err != nil {
		t.Fatalf("repo root %s: %v", root, err)
	}
	return root
}
