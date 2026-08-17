package model

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitSQLDoesNotContainEdgesAggMV(t *testing.T) {
	raw := readRepoFile(t, "clickhouse", "init.sql")
	if strings.Contains(raw, "traffic_edges_daily_mv") {
		t.Error("init.sql must not contain traffic_edges_daily_mv")
	}
	if strings.Contains(raw, "CREATE MATERIALIZED VIEW") && strings.Contains(strings.ToLower(raw), "traffic_edges") {
		t.Error("init.sql must not CREATE MATERIALIZED VIEW for traffic_edges")
	}
}

func TestGeneratedSQLContainsActionVocab(t *testing.T) {
	allowed := AllowedInClause()
	blocked := BlockedInClause()
	initSQL := readRepoFile(t, "clickhouse", "init.sql")
	if !strings.Contains(initSQL, allowed) {
		t.Error("init.sql must contain AllowedInClause (regenerate migrate schema)")
	}
	success := readRepoFile(t, "clickhouse", "migrate_success.sql")
	if !strings.Contains(success, allowed) {
		t.Error("migrate_success.sql must contain AllowedInClause")
	}
	edges := readRepoFile(t, "clickhouse", "migrate_edges_agg.sql")
	if !strings.Contains(edges, blocked) {
		t.Error("migrate_edges_agg.sql must contain BlockedInClause")
	}
	if n := strings.Count(edges, blocked); n < 2 {
		t.Errorf("migrate_edges_agg.sql: blocked clause appears %d times, want >= 2", n)
	}
}

func TestActionVocabBlockedExactInOpsShell(t *testing.T) {
	want := clauseTokenSet(t, BlockedInClause())
	sh := readRepoFile(t, "clickhouse", "backfill_edges_agg.sh")
	shInners, err := ExtractMarkedInner(sh, markerBlockedSHBegin, markerBlockedSHEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(shInners) != 1 {
		t.Fatalf("backfill_edges_agg.sh: want 1 BLOCKED region, got %d", len(shInners))
	}
	line := strings.TrimSpace(shInners[0])
	const prefix = `BLOCKED="`
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, `"`) {
		t.Fatalf("backfill BLOCKED line = %q", line)
	}
	clause := strings.TrimSuffix(strings.TrimPrefix(line, prefix), `"`)
	assertSameTokenSet(t, "backfill_edges_agg.sh", clauseTokenSet(t, clause), want)
}

func TestActionVocabOpsUpToDate(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range ActionVocabOpsRelPaths {
		path := filepath.Join(root, rel)
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		next, err := ApplyActionVocabMarkers(rel, string(raw))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		got := strings.ReplaceAll(next, "\r\n", "\n")
		want := strings.ReplaceAll(string(raw), "\r\n", "\n")
		if got != want {
			t.Errorf("%s is out of date; run: cd backend && go generate ./internal/model/...", rel)
		}
	}
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{repoRoot(t)}, parts...)...)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(raw)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "clickhouse", "init.sql")); err != nil {
		t.Fatalf("repo root %s: %v", root, err)
	}
	return root
}

func clauseTokenSet(t *testing.T, clause string) map[string]struct{} {
	t.Helper()
	out := make(map[string]struct{})
	for _, part := range strings.Split(clause, ",") {
		tok := strings.TrimSpace(part)
		if tok == "" {
			continue
		}
		if len(tok) < 2 || tok[0] != '\'' || tok[len(tok)-1] != '\'' {
			t.Fatalf("bad quoted token %q in %q", tok, clause)
		}
		out[tok[1:len(tok)-1]] = struct{}{}
	}
	if len(out) == 0 {
		t.Fatalf("empty token set from %q", clause)
	}
	return out
}

func assertSameTokenSet(t *testing.T, label string, got, want map[string]struct{}) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: len got=%d want=%d", label, len(got), len(want))
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Errorf("%s: missing %q", label, k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: unexpected %q", label, k)
		}
	}
}
