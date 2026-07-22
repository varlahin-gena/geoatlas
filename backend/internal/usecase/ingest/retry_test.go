package ingest

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsRetryableInsertError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{context.Canceled, false},
		{context.DeadlineExceeded, false},
		{errInsertCircuitOpen, false},
		{errors.New("syntax error"), false},
		{errors.New("code: 60"), false},
		{errors.New("connection refused"), true},
		{errors.New("i/o timeout"), true},
		{errors.New("weird unknown"), true},
	}
	for _, tc := range cases {
		if got := isRetryableInsertError(tc.err); got != tc.want {
			t.Errorf("%v: got %v want %v", tc.err, got, tc.want)
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
	err := insertWithRetry(context.Background(), time.Second, "t", func(ctx context.Context) error {
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

func TestInsertCircuitBlocks(t *testing.T) {
	ins := &stubInserter{trafficErr: errors.New("connection refused")}
	proc := NewProcessor(testDeps(ins), nil)
	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=2`
	_, _, _ = proc.ProcessLine(context.Background(), line, "")
	proc.insertFails = circuitFailThreshold
	proc.circuitOpenUntil = time.Now().Add(time.Minute)
	err := proc.ForceFlushLogs(context.Background())
	if !errors.Is(err, errInsertCircuitOpen) {
		t.Fatalf("err=%v want circuit open", err)
	}
}
