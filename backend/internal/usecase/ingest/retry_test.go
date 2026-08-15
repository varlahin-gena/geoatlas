package ingest

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestShouldRetryInsert(t *testing.T) {
	always := InsertErrorClassifyFunc(func(error) bool { return true })
	never := InsertErrorClassifyFunc(func(error) bool { return false })
	cases := []struct {
		name      string
		err       error
		classify  InsertErrorClassifier
		wantRetry bool
	}{
		{"nil", nil, always, false},
		{"canceled", context.Canceled, always, false},
		{"deadline", context.DeadlineExceeded, always, false},
		{"circuit", errInsertCircuitOpen, always, false},
		{"nil classify retries", errors.New("x"), nil, true},
		{"classify false", errors.New("x"), never, false},
		{"classify true", errors.New("x"), always, true},
	}
	for _, tc := range cases {
		if got := shouldRetryInsert(tc.err, tc.classify); got != tc.wantRetry {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.wantRetry)
		}
	}
}

func TestInsertWithRetrySucceedsAfterTransient(t *testing.T) {
	oldA, oldB := insertRetryAttempts, insertRetryBase
	insertRetryAttempts = 3
	insertRetryBase = time.Millisecond
	t.Cleanup(func() {
		insertRetryAttempts = oldA
		insertRetryBase = oldB
	})

	n := 0
	err := insertWithRetry(context.Background(), time.Second, "t", nil, func(ctx context.Context) error {
		n++
		if n < 2 {
			return errors.New("connection reset")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("attempts=%d", n)
	}
}

func TestInsertWithRetryStopsWhenNotRetryable(t *testing.T) {
	oldA, oldB := insertRetryAttempts, insertRetryBase
	insertRetryAttempts = 5
	insertRetryBase = time.Millisecond
	t.Cleanup(func() {
		insertRetryAttempts = oldA
		insertRetryBase = oldB
	})

	n := 0
	never := InsertErrorClassifyFunc(func(error) bool { return false })
	err := insertWithRetry(context.Background(), time.Second, "t", never, func(ctx context.Context) error {
		n++
		return errors.New("permanent")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if n != 1 {
		t.Fatalf("attempts=%d want 1", n)
	}
}

func TestInsertCircuitBlocks(t *testing.T) {
	ins := &stubInserter{trafficErr: errors.New("connection refused")}
	cb := NewCircuitBreaker()
	cb.OpenForTest(time.Minute)
	d := testDeps(ins)
	d.Circuit = cb
	proc := NewProcessor(d, nil)
	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=2`
	_, _, _ = proc.ProcessLine(context.Background(), line, "")
	err := proc.ForceFlushLogs(context.Background())
	if !errors.Is(err, errInsertCircuitOpen) {
		t.Fatalf("err=%v want circuit open", err)
	}
}

func TestSharedCircuitAcrossProcessors(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < circuitFailThreshold; i++ {
		cb.NoteFailure()
	}
	d1 := testDeps(&stubInserter{})
	d1.Circuit = cb
	d2 := testDeps(&stubInserter{})
	d2.Circuit = cb
	p1 := NewProcessor(d1, nil)
	p2 := NewProcessor(d2, nil)

	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=2`
	_, _, _ = p1.ProcessLine(context.Background(), line, "")
	_, _, _ = p2.ProcessLine(context.Background(), line, "")

	if err := p1.ForceFlushLogs(context.Background()); !errors.Is(err, errInsertCircuitOpen) {
		t.Fatalf("p1 err=%v want circuit open", err)
	}
	if err := p2.ForceFlushLogs(context.Background()); !errors.Is(err, errInsertCircuitOpen) {
		t.Fatalf("p2 err=%v want circuit open", err)
	}
}
