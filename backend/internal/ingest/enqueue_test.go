package ingest

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestEnqueueDropsWhenQueueFull(t *testing.T) {
	svc := &Service{
		cfg:    Config{ConnIdleTimeout: time.Second},
		lineCh: make(chan ingestedLine, 1),
		stats:  newStats(),
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
