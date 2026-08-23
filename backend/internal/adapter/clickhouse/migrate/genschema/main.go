// Command genschema writes clickhouse/init.sql and migrate_*.sql from migrate.Render*.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"geoatlas/internal/adapter/clickhouse/migrate"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "genschema: %v\n", err)
		os.Exit(1)
	}
	files := []struct {
		rel  string
		body string
	}{
		{filepath.Join("clickhouse", "init.sql"), migrate.RenderColdBootstrapSQL()},
		{filepath.Join("clickhouse", "migrate_success.sql"), migrate.RenderMigrateSuccessSQL()},
		{filepath.Join("clickhouse", "migrate_edges_agg.sql"), migrate.RenderMigrateEdgesAggSQL()},
	}
	for _, f := range files {
		path, err := confinedRepoPath(root, f.rel)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genschema: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(path, []byte(f.body), 0o644); err != nil { //nolint:gosec // G703: path confined to repoRoot
			fmt.Fprintf(os.Stderr, "genschema: write %s: %v\n", path, err)
			os.Exit(1)
		}
		fmt.Println("wrote", f.rel)
	}
}

func repoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	// …/backend/internal/adapter/clickhouse/migrate/genschema/main.go → repo root
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "clickhouse")); err != nil {
		return "", fmt.Errorf("repo root %s: %w", root, err)
	}
	return root, nil
}

func confinedRepoPath(repoRoot, rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("invalid rel path %q", rel)
	}
	path := filepath.Clean(filepath.Join(repoRoot, rel))
	relOut, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return "", fmt.Errorf("rel %s: %w", path, err)
	}
	if relOut == ".." || strings.HasPrefix(relOut, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repo root: %s", rel)
	}
	return path, nil
}
