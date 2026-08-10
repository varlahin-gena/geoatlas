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
	// Edges MV/table — SoT в migrate.Ensure*; init.sql только cold bootstrap базовых таблиц.
	if strings.Contains(raw, "traffic_edges_daily_mv") {
		t.Error("init.sql must not contain traffic_edges_daily_mv")
	}
	if strings.Contains(raw, "CREATE MATERIALIZED VIEW") && strings.Contains(strings.ToLower(raw), "traffic_edges") {
		t.Error("init.sql must not CREATE MATERIALIZED VIEW for traffic_edges")
	}
}

func TestActionVocabAllowedExactInOpsSQL(t *testing.T) {
	want := clauseTokenSet(t, AllowedInClause())
	for _, name := range []string{"init.sql", "migrate_success.sql"} {
		raw := readRepoFile(t, "clickhouse", name)
		inners, err := ExtractMarkedInner(raw, markerAllowedBegin, markerAllowedEnd)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(inners) != 1 {
			t.Fatalf("%s: want 1 ALLOWED region, got %d", name, len(inners))
		}
		got := clauseTokenSet(t, inners[0])
		assertSameTokenSet(t, name+" ALLOWED", got, want)
		// success list must not include blocked verbs
		for tok := range clauseTokenSet(t, BlockedInClause()) {
			if _, ok := got[tok]; ok {
				t.Errorf("%s success list unexpectedly contains blocked %q", name, tok)
			}
		}
	}
}

func TestActionVocabBlockedExactInOps(t *testing.T) {
	want := clauseTokenSet(t, BlockedInClause())

	edges := readRepoFile(t, "clickhouse", "migrate_edges_agg.sql")
	inners, err := ExtractMarkedInner(edges, markerBlockedBegin, markerBlockedEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(inners) != 2 {
		t.Fatalf("migrate_edges_agg.sql: want 2 BLOCKED regions, got %d", len(inners))
	}
	for _, inner := range inners {
		got := clauseTokenSet(t, inner)
		assertSameTokenSet(t, "migrate_edges_agg.sql BLOCKED", got, want)
	}

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
