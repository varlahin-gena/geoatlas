package chexchange

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type memConn struct {
	calls []string
	fail  map[string]error // substring → error
}

func (m *memConn) Exec(_ context.Context, query string, _ ...any) error {
	m.calls = append(m.calls, query)
	for sub, err := range m.fail {
		if strings.Contains(query, sub) {
			return err
		}
	}
	return nil
}

func TestRebuildViaNextOrder(t *testing.T) {
	t.Parallel()
	var calls []string
	exec := func(_ context.Context, q string) error {
		calls = append(calls, q)
		return nil
	}
	if err := RebuildViaNext(context.Background(), exec, "live", "live__next", "CREATE TABLE live__next (...)"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"DROP TABLE IF EXISTS live__next",
		"CREATE TABLE live__next (...)",
		"EXCHANGE TABLES live AND live__next",
		"DROP TABLE IF EXISTS live__next",
	}
	if len(calls) != len(want) {
		t.Fatalf("calls = %#v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("call[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestRebuildViaNextCreateFailDropsNext(t *testing.T) {
	t.Parallel()
	var calls []string
	exec := func(_ context.Context, q string) error {
		calls = append(calls, q)
		if strings.HasPrefix(q, "CREATE ") {
			return errors.New("boom")
		}
		return nil
	}
	err := RebuildViaNext(context.Background(), exec, "t", "t__next", "CREATE TABLE t__next AS x")
	if err == nil {
		t.Fatal("expected error")
	}
	if calls[len(calls)-1] != "DROP TABLE IF EXISTS t__next" {
		t.Fatalf("last call = %q", calls[len(calls)-1])
	}
}

func TestReplaceViaStaging(t *testing.T) {
	t.Parallel()
	ch := &memConn{}
	filled := false
	err := ReplaceViaStaging(context.Background(), ch, "geo_ranges", "geo_ranges__staging", func(context.Context) error {
		filled = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !filled {
		t.Fatal("fill not called")
	}
	joined := strings.Join(ch.calls, " | ")
	for _, part := range []string{
		"DROP TABLE IF EXISTS geo_ranges__staging",
		"CREATE TABLE geo_ranges__staging AS geo_ranges",
		"EXCHANGE TABLES geo_ranges AND geo_ranges__staging",
	} {
		if !strings.Contains(joined, part) {
			t.Fatalf("missing %q in %s", part, joined)
		}
	}
	// final drop of staging (old data)
	if ch.calls[len(ch.calls)-1] != "DROP TABLE IF EXISTS geo_ranges__staging" {
		t.Fatalf("last = %q", ch.calls[len(ch.calls)-1])
	}
}

func TestReplaceViaStagingFillFail(t *testing.T) {
	t.Parallel()
	ch := &memConn{}
	err := ReplaceViaStaging(context.Background(), ch, "live", "stg", func(context.Context) error {
		return errors.New("insert failed")
	})
	if err == nil || err.Error() != "insert failed" {
		t.Fatalf("err = %v", err)
	}
	if ch.calls[len(ch.calls)-1] != "DROP TABLE IF EXISTS stg" {
		t.Fatalf("staging not dropped after fill fail: %#v", ch.calls)
	}
}

func TestSwapAndDropRejectsEmpty(t *testing.T) {
	t.Parallel()
	err := SwapAndDrop(context.Background(), func(context.Context, string) error { return nil }, "", "x")
	if err == nil {
		t.Fatal("expected error")
	}
}
