package ingestnet

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"
)

func TestEnqueueDropsWhenQueueFull(t *testing.T) {
	svc := &Service{
		cfg:     Config{ConnIdleTimeout: time.Second},
		lineCh:  make(chan ingestedLine, 1),
		stats:   newStats(),
		connSem: make(chan struct{}, 1),
	}
	svc.lineCh <- ingestedLine{line: "already-buffered", transport: "tcp"}

	client, server := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.wg.Add(1)
		svc.connSem <- struct{}{}
		svc.handleConn(context.Background(), server, "tcp")
	}()

	// Одна строка сверх capacity → drop, TCP не должен зависнуть.
	payload := []byte("over-capacity-line\n")
	_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = client.Close() // EOF for reader

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleConn blocked on full queue")
	}

	if got := svc.stats.droppedTotal.Load(); got < 1 {
		t.Fatalf("dropped_total=%d want >= 1", got)
	}
	if len(svc.lineCh) != 1 {
		t.Fatalf("queue len=%d want 1 (original item kept)", len(svc.lineCh))
	}
}

func TestEnqueueAcceptsWhenSpace(t *testing.T) {
	svc := &Service{
		cfg:     Config{ConnIdleTimeout: time.Second},
		lineCh:  make(chan ingestedLine, 4),
		stats:   newStats(),
		connSem: make(chan struct{}, 1),
	}
	client, server := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.wg.Add(1)
		svc.connSem <- struct{}{}
		svc.handleConn(context.Background(), server, "udp")
	}()

	_ = client.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := client.Write([]byte("ok-line\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = client.Close()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handleConn hung")
	}

	if svc.stats.droppedTotal.Load() != 0 {
		t.Fatalf("unexpected drops")
	}
	select {
	case item := <-svc.lineCh:
		if item.line != "ok-line" {
			t.Fatalf("line=%q", item.line)
		}
	default:
		t.Fatal("expected enqueued line")
	}
}

func TestEnqueueFloodDropsUnderTinyQueue(t *testing.T) {
	const queueCap = 8
	const floodLines = 200

	svc := &Service{
		cfg:     Config{ConnIdleTimeout: 2 * time.Second},
		lineCh:  make(chan ingestedLine, queueCap),
		stats:   newStats(),
		connSem: make(chan struct{}, 1),
	}

	client, server := net.Pipe()
	defer client.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		svc.wg.Add(1)
		svc.connSem <- struct{}{}
		svc.handleConn(context.Background(), server, "tcp")
	}()

	_ = client.SetWriteDeadline(time.Now().Add(5 * time.Second))
	var payload []byte
	for i := 0; i < floodLines; i++ {
		payload = append(payload, []byte("flood-line\n")...)
	}
	if _, err := client.Write(payload); err != nil {
		t.Fatalf("write flood: %v", err)
	}
	_ = client.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleConn hung under flood (expected non-blocking drops)")
	}

	dropped := svc.stats.droppedTotal.Load()
	if dropped < int64(floodLines-queueCap) {
		t.Fatalf("dropped_total=%d want >= %d (queue=%d flood=%d)",
			dropped, floodLines-queueCap, queueCap, floodLines)
	}
	depth := len(svc.lineCh)
	if depth != queueCap {
		t.Fatalf("queue depth=%d want %d (full)", depth, queueCap)
	}
	snap := svc.Stats()
	if snap.DroppedTotal != dropped {
		t.Fatalf("Stats().DroppedTotal=%d want %d", snap.DroppedTotal, dropped)
	}
}

func TestEnqueueDropsWhenByteBudgetFull(t *testing.T) {
	svc := &Service{
		cfg:    Config{QueueMaxBytes: 20},
		lineCh: make(chan ingestedLine, 100), // depth intentionally large
		stats:  newStats(),
	}

	if !svc.TryEnqueue("abcdefghij", "tcp") { // 10 bytes
		t.Fatal("first enqueue should succeed")
	}
	if !svc.TryEnqueue("1234567890", "tcp") { // 10 bytes → exactly 20
		t.Fatal("second enqueue should succeed")
	}
	if svc.TryEnqueue("x", "tcp") {
		t.Fatal("third enqueue should drop on byte budget")
	}
	if got := svc.stats.droppedTotal.Load(); got != 1 {
		t.Fatalf("dropped=%d want 1", got)
	}
	if got := svc.queueBytes.Load(); got != 20 {
		t.Fatalf("queue_bytes=%d want 20", got)
	}
	snap := svc.Stats()
	if snap.QueueBytes != 20 || snap.QueueBytesCapacity != 20 {
		t.Fatalf("stats bytes=%d cap=%d", snap.QueueBytes, snap.QueueBytesCapacity)
	}

	// Dequeue releases budget.
	item := <-svc.lineCh
	svc.releaseQueueBytes(item.line)
	if !svc.TryEnqueue("abcdefghij", "tcp") {
		t.Fatal("enqueue after release should succeed")
	}
	if got := svc.queueBytes.Load(); got != 20 {
		t.Fatalf("after release+enqueue queue_bytes=%d want 20", got)
	}
}

func TestEnqueueDropsOversizedLine(t *testing.T) {
	svc := &Service{
		cfg:    Config{QueueMaxBytes: 8},
		lineCh: make(chan ingestedLine, 10),
		stats:  newStats(),
	}
	if svc.TryEnqueue("0123456789", "udp") {
		t.Fatal("line larger than budget must drop")
	}
	if len(svc.lineCh) != 0 || svc.queueBytes.Load() != 0 {
		t.Fatal("oversized drop must not reserve bytes or occupy queue")
	}
}

func TestEnqueueConcurrentByteBudget(t *testing.T) {
	const (
		workers   = 8
		perWorker = 5000
		line      = "src=10.0.0.1 dst=8.8.8.8 action=allow" // 35 bytes
		budget    = 35 * 200                                // ~200 lines worth
	)
	svc := &Service{
		cfg:    Config{QueueMaxBytes: budget},
		lineCh: make(chan ingestedLine, 10000), // depth >> byte budget
		stats:  newStats(),
	}

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				_ = svc.TryEnqueue(line, "tcp")
			}
		}()
	}
	wg.Wait()

	snap := svc.Stats()
	if snap.QueueBytes > int64(budget) {
		t.Fatalf("queue_bytes=%d exceeds budget=%d", snap.QueueBytes, budget)
	}
	if snap.DroppedTotal < 1 {
		t.Fatal("expected concurrent flood to drop on byte budget")
	}
	// Depth can be at most budget/len(line); allow +1 race slack none — exact cap.
	maxDepth := budget / len(line)
	if snap.QueueDepth > int64(maxDepth) {
		t.Fatalf("queue_depth=%d want <= %d under byte budget", snap.QueueDepth, maxDepth)
	}
	t.Logf("concurrent soak: depth=%d bytes=%d/%d dropped=%d",
		snap.QueueDepth, snap.QueueBytes, snap.QueueBytesCapacity, snap.DroppedTotal)
}

func BenchmarkTryEnqueueByteBudget(b *testing.B) {
	svc := &Service{
		cfg:    Config{QueueMaxBytes: 1 << 20}, // 1 MiB
		lineCh: make(chan ingestedLine, 100000),
		stats:  newStats(),
	}
	line := "src=10.0.0.1 dst=8.8.8.8 action=allow proto=tcp sport=1 dport=443"
	// Drain in background so enqueue doesn't always hit depth first.
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case item := <-svc.lineCh:
				svc.releaseQueueBytes(item.line)
			}
		}
	}()
	b.Cleanup(func() { close(done) })

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = svc.TryEnqueue(line, "tcp")
	}
}
