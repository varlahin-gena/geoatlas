package ingestnet

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"geoatlas/internal/adapter/parseradapter"
	"geoatlas/internal/model"
	"geoatlas/internal/parser"
	usecaseingest "geoatlas/internal/usecase/ingest"
)

func testLineParser() usecaseingest.LineParser {
	return parseradapter.New(parser.NewRegistry(
		&parser.UserGateCEF{},
		&parser.FortigateCEF{},
		&parser.CiscoFTD{},
		&parser.CiscoASA{},
		&parser.CowrieJSON{},
		&parser.GenericKV{},
	))
}

func testProcDeps() ProcessorDeps {
	return ProcessorDeps{Parser: testLineParser()}
}

func TestDrainWorkerReturnsOnEmptyQueue(t *testing.T) {
	st := newStats()
	ucDeps := usecaseingest.Deps{Parser: testLineParser(), BatchSize: 10, QueryTimeout: time.Second}
	proc := usecaseingest.NewProcessor(ucDeps, st)
	svc := &Service{
		cfg:    Config{QueryTimeout: time.Second},
		lineCh: make(chan ingestedLine, 4),
		stats:  st,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		svc.drainWorker(ctx, proc)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("drainWorker hung")
	}
}

func TestDrainTimeoutScalesWithQueue(t *testing.T) {
	svc := &Service{
		cfg: Config{
			Workers:       2,
			BatchSize:     100,
			FlushInterval: time.Second,
			QueryTimeout:  3 * time.Minute, // не должен стать базой drain
		},
		lineCh: make(chan ingestedLine, 10000),
	}
	empty := svc.drainTimeout()
	if empty != 30*time.Second {
		t.Fatalf("empty queue drain=%s want 30s", empty)
	}
	for i := 0; i < 5000; i++ {
		svc.lineCh <- ingestedLine{line: "x"}
	}
	got := svc.drainTimeout()
	if got <= empty {
		t.Fatalf("deep queue drain=%s should exceed empty=%s", got, empty)
	}
	if got > 2*time.Minute {
		t.Fatalf("drain=%s exceeds 2m cap", got)
	}
	wait := svc.ShutdownWaitTimeout()
	if wait <= got {
		t.Fatalf("ShutdownWaitTimeout=%s should exceed drainTimeout=%s", wait, got)
	}
}

func TestIngestRunConnLimitAndShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	svc := NewService(Config{
		Bindings:        []Binding{{Addr: addr, Transport: "tcp"}},
		Workers:         1,
		BatchSize:       100,
		QueueSize:       16,
		MaxConnections:  1,
		FlushInterval:   time.Hour,
		QueryTimeout:    time.Second,
		ConnIdleTimeout: 2 * time.Second,
	}, testProcDeps())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	var c1 net.Conn
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c1, err = net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if c1 == nil {
		cancel()
		t.Fatal("could not dial ingest listener")
	}
	defer c1.Close()

	time.Sleep(50 * time.Millisecond)
	c2, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		cancel()
		t.Fatalf("second dial: %v", err)
	}
	_ = c2.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 1)
	_, rerr := c2.Read(buf)
	_ = c2.Close()
	if rerr == nil {
		t.Fatal("expected rejected connection to close (EOF/reset)")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestIngestRunHTTPOnlyNoBindings(t *testing.T) {
	svc := NewService(Config{Workers: 1, FlushInterval: time.Hour}, testProcDeps())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	if snap := svc.Stats(); snap.State != "running" {
		cancel()
		t.Fatalf("state=%q want running", snap.State)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestQueueBlocksUntilDrain(t *testing.T) {
	st := newStats()
	ucDeps := usecaseingest.Deps{Parser: testLineParser(), BatchSize: 100, QueryTimeout: time.Second}
	proc := usecaseingest.NewProcessor(ucDeps, st)
	svc := &Service{
		cfg:    Config{QueryTimeout: time.Second},
		lineCh: make(chan ingestedLine, 1),
		stats:  st,
	}
	svc.lineCh <- ingestedLine{line: "not-a-valid-firewall-line-xyz", transport: "tcp"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	svc.drainWorker(ctx, proc)

	select {
	case <-svc.lineCh:
		t.Fatal("expected queue drained")
	default:
	}
}

func TestRunDrainsDeepQueueOnCancel(t *testing.T) {
	sink := &countingInserter{}
	svc := NewService(Config{
		Workers:       2,
		BatchSize:     50,
		QueueSize:     500,
		FlushInterval: time.Hour,
		QueryTimeout:  time.Second,
	}, ProcessorDeps{
		Parser: testLineParser(),
		Logs:   sink,
		Errors: sink,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=2`
	const n = 400
	for i := 0; i < n; i++ {
		if !svc.TryEnqueue(line, "tcp") {
			cancel()
			t.Fatalf("enqueue failed at %d", i)
		}
	}

	// Даём worker'ам стартовать, затем cancel → drainWorker должен опустошить хвост.
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not finish after cancel (drain hung?)")
	}

	if left := len(svc.lineCh); left != 0 {
		t.Fatalf("queue_depth=%d want 0 after drain", left)
	}
	if got := sink.logs.Load(); got < 1 {
		t.Fatalf("inserted=%d want >= 1", got)
	}
}

func TestAbortDrainCancelsInFlightDrain(t *testing.T) {
	svc := NewService(Config{
		Workers:       1,
		BatchSize:     10,
		QueueSize:     8,
		FlushInterval: time.Hour,
		QueryTimeout:  time.Second,
	}, testProcDeps())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	// Дождаться готовности drainRoot.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.drainMu.Lock()
		ready := svc.drainCancel != nil
		svc.drainMu.Unlock()
		if ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	drainCtx, drainCancel := svc.beginDrain(ctx)
	defer drainCancel()

	svc.AbortDrain()
	select {
	case <-drainCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("AbortDrain did not cancel drain context")
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run hung after AbortDrain+cancel")
	}
}

func TestAbortDrainStopsBlockedDrainBeforeClose(t *testing.T) {
	// Медленный insert: drain не успеет закончить до AbortDrain.
	block := make(chan struct{})
	ins := &blockingInserter{block: block}
	deps := testProcDeps()
	deps.Logs = ins
	deps.Errors = ins

	svc := NewService(Config{
		Workers:       1,
		BatchSize:     2,
		QueueSize:     64,
		QueueMaxBytes: 1 << 20,
		FlushInterval: time.Hour,
		QueryTimeout:  30 * time.Second,
	}, deps)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		svc.drainMu.Lock()
		ready := svc.drainCancel != nil
		svc.drainMu.Unlock()
		if ready {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=2`
	for i := 0; i < 10; i++ {
		if !svc.TryEnqueue(line, "tcp") {
			t.Fatal("enqueue failed")
		}
	}

	cancel() // workers enter drain → ProcessLine → Flush → block on insert
	time.Sleep(50 * time.Millisecond)
	svc.AbortDrain()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		close(block) // unblock so test doesn't leak goroutine forever
		t.Fatal("Run did not finish after AbortDrain (would race pools.Close)")
	}
	close(block)
}

func TestIsClosedConnTyped(t *testing.T) {
	if !isClosedConn(net.ErrClosed) {
		t.Fatal("net.ErrClosed should match")
	}
	if isClosedConn(errors.New("something else")) {
		t.Fatal("unrelated error must not match")
	}
	if !isClosedConn(errors.New("read: connection reset by peer")) {
		t.Fatal("connection reset fallback")
	}
}

type blockingInserter struct {
	block chan struct{}
}

func (b *blockingInserter) InsertTrafficLogs(ctx context.Context, _ []model.TrafficLog) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.block:
		return nil
	}
}

func (b *blockingInserter) InsertParseErrors(ctx context.Context, _ []model.ParseError) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-b.block:
		return nil
	}
}

type countingInserter struct {
	logs atomic.Int64
}

func (c *countingInserter) InsertTrafficLogs(_ context.Context, logs []model.TrafficLog) error {
	c.logs.Add(int64(len(logs)))
	return nil
}

func (c *countingInserter) InsertParseErrors(context.Context, []model.ParseError) error {
	return nil
}

func TestWorkerPausesDequeueWhenCircuitOpen(t *testing.T) {
	ins := &countingInserter{}
	circuit := usecaseingest.NewCircuitBreaker()
	circuit.OpenForTest(2 * time.Second)

	svc := NewService(Config{
		Workers:       1,
		BatchSize:     10,
		FlushInterval: 50 * time.Millisecond,
		QueueSize:     64,
		QueryTimeout:  time.Second,
	}, ProcessorDeps{Logs: ins, Errors: ins, Parser: testLineParser()})
	svc.circuit = circuit
	for i := range svc.processors {
		svc.processors[i] = usecaseingest.NewProcessor(usecaseingest.Deps{
			Logs: ins, Errors: ins, Parser: testLineParser(),
			BatchSize: 10, QueryTimeout: time.Second, Circuit: circuit,
		}, svc.stats)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.Run(ctx) }()

	line := `src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=443`
	const n = 20
	for i := 0; i < n; i++ {
		if !svc.TryEnqueue(line, "tcp") {
			t.Fatalf("enqueue %d failed", i)
		}
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		snap := svc.Stats()
		if !snap.CircuitOpen {
			t.Fatal("circuit should stay open during wait")
		}
		if snap.QueueDepth < int64(n) {
			t.Fatalf("dequeue advanced while circuit open: depth=%d want %d", snap.QueueDepth, n)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if ins.logs.Load() != 0 {
		t.Fatalf("inserted=%d want 0 while circuit open", ins.logs.Load())
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop")
	}
}

