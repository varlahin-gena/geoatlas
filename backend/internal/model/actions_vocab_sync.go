package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	markerBlockedSHBegin = "# ACTION_VOCAB:BLOCKED_BEGIN"
	markerBlockedSHEnd   = "# ACTION_VOCAB:BLOCKED_END"
)

// ActionVocabOpsRelPaths — ops-артефакты с помеченными списками (не runtime SoT).
// SQL под clickhouse/ генерируется целиком из migrate.Render* (go generate migrate).
var ActionVocabOpsRelPaths = []string{
	filepath.Join("clickhouse", "backfill_edges_agg.sh"),
}

// ApplyActionVocabMarkers подставляет AllowedInClause/BlockedInClause между маркерами.
func ApplyActionVocabMarkers(relPath, content string) (string, error) {
	base := filepath.Base(relPath)
	switch base {
	case "backfill_edges_agg.sh":
		body := `BLOCKED="` + BlockedInClause() + `"`
		return replaceMarkedRegions(content, markerBlockedSHBegin, markerBlockedSHEnd, body)
	default:
		return "", fmt.Errorf("unsupported action-vocab ops file: %s", relPath)
	}
}

// SyncActionVocabOps переписывает помеченные регионы в clickhouse/ ops-файлах.
func SyncActionVocabOps(repoRoot string) error {
	root, err := filepath.Abs(filepath.Clean(repoRoot))
	if err != nil {
		return fmt.Errorf("repo root: %w", err)
	}
	for _, rel := range ActionVocabOpsRelPaths {
		path, err := confinedRepoPath(root, rel)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path) //nolint:gosec // G304/G703: path confined to repoRoot + fixed rel allowlist
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		next, err := ApplyActionVocabMarkers(rel, string(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if strings.ReplaceAll(next, "\r\n", "\n") == strings.ReplaceAll(string(raw), "\r\n", "\n") {
			continue
		}
		if err := os.WriteFile(path, []byte(next), 0o600); err != nil { //nolint:gosec // G703: path confined to repoRoot + fixed rel allowlist
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

// confinedRepoPath joins root+rel and rejects anything that escapes root.
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

func replaceMarkedRegions(src, begin, end, inner string) (string, error) {
	if !strings.Contains(src, begin) || !strings.Contains(src, end) {
		return "", fmt.Errorf("missing markers %q … %q", begin, end)
	}
	var b strings.Builder
	rest := src
	replaced := 0
	for {
		start := strings.Index(rest, begin)
		if start < 0 {
			b.WriteString(rest)
			break
		}
		afterBegin := start + len(begin)
		endRel := strings.Index(rest[afterBegin:], end)
		if endRel < 0 {
			return "", fmt.Errorf("marker %q without closing %q", begin, end)
		}
		endAt := afterBegin + endRel
		b.WriteString(rest[:afterBegin])
		// Keep a single newline around generated content when markers sit on their own lines.
		b.WriteByte('\n')
		b.WriteString(inner)
		b.WriteByte('\n')
		b.WriteString(rest[endAt : endAt+len(end)])
		rest = rest[endAt+len(end):]
		replaced++
	}
	if replaced == 0 {
		return "", fmt.Errorf("no regions replaced for %q", begin)
	}
	return b.String(), nil
}

// ExtractMarkedInner returns concatenated inners for begin/end (all occurrences).
func ExtractMarkedInner(src, begin, end string) ([]string, error) {
	var out []string
	rest := src
	for {
		start := strings.Index(rest, begin)
		if start < 0 {
			break
		}
		afterBegin := start + len(begin)
		endRel := strings.Index(rest[afterBegin:], end)
		if endRel < 0 {
			return nil, fmt.Errorf("marker %q without closing %q", begin, end)
		}
		inner := strings.TrimSpace(rest[afterBegin : afterBegin+endRel])
		out = append(out, inner)
		rest = rest[afterBegin+endRel+len(end):]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("markers %q … %q not found", begin, end)
	}
	return out, nil
}
