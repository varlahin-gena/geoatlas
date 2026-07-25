package ingest

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
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
		{errors.New("code: 60, message: Table x doesn't exist"), false},
		{errors.New("code: 516, message: Authentication failed"), false},
		{&clickhouse.Exception{Code: 60, Message: "UNKNOWN_TABLE"}, false},
		{&clickhouse.Exception{Code: 241, Message: "Memory limit exceeded"}, true},
		{errors.New("code: 241, message: Memory limit exceeded"), true},
		{errors.New("connection refused"), true},
		{errors.New("i/o timeout"), true},
		{errors.New("weird unknown"), true},
		{&net.OpError{Op: "dial", Err: errors.New("connection refused")}, true},
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
