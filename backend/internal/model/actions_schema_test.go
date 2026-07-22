package model

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestInitSQLSuccessMatchesAllowedInClause(t *testing.T) {
	initSQL := readRepoFile(t, "clickhouse", "init.sql")
	assertSuccessMaterializedMatchesAllowed(t, initSQL)
}

func TestMigrateEdgesAggBlockedMatchesModel(t *testing.T) {
	raw := readRepoFile(t, "clickhouse", "migrate_edges_agg.sql")
	blocked := BlockedInClause()
	for _, item := range strings.Split(blocked, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(raw, item) {
			t.Errorf("migrate_edges_agg.sql missing blocked action %s", item)
		}
	}
}

func TestInitSQLDoesNotContainEdgesAggMV(t *testing.T) {
	raw := readRepoFile(t, "clickhouse", "init.sql")
	// Edges MV/table — SoT в storage.Ensure*; init.sql только cold bootstrap базовых таблиц.
	if strings.Contains(raw, "traffic_edges_daily_mv") {
		t.Error("init.sql must not contain traffic_edges_daily_mv")
	}
	if strings.Contains(raw, "CREATE MATERIALIZED VIEW") && strings.Contains(strings.ToLower(raw), "traffic_edges") {
		t.Error("init.sql must not CREATE MATERIALIZED VIEW for traffic_edges")
	}
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	args := append([]string{filepath.Dir(thisFile), "..", "..", ".."}, parts...)
	path := filepath.Clean(filepath.Join(args...))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func assertSuccessMaterializedMatchesAllowed(t *testing.T, raw string) {
	t.Helper()
	re := regexp.MustCompile(`(?s)success\s+UInt8 MATERIALIZED\s+if\(lower\(action\) IN \((.*?)\),\s*1,\s*0\)`)
	m := re.FindStringSubmatch(raw)
	if m == nil {
		t.Fatal("success MATERIALIZED IN (...) not found in init.sql")
	}
	sqlClause := m[1]

	for _, item := range strings.Split(AllowedInClause(), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !strings.Contains(sqlClause, item) {
			t.Errorf("init.sql success list missing %s (from model.AllowedInClause)", item)
		}
	}

	for _, item := range strings.Split(BlockedInClause(), ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(sqlClause, item) {
			t.Errorf("init.sql success list unexpectedly contains blocked action %s", item)
		}
	}
}
