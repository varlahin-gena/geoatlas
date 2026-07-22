package ingest

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestTryEnqueueDropsWhenFull(t *testing.T) {
	svc := &Service{
		cfg:    Config{FlushInterval: 50 * time.Millisecond},
		lineCh: make(chan ingestedLine, 1),
		stats:  newStats(),
	}
	svc.lineCh <- ingestedLine{line: "kept", transport: "tcp"}

	if svc.TryEnqueue("drop-me", "http") {
		t.Fatal("expected drop on full queue")
	}
	if svc.stats.droppedTotal.Load() != 1 {
		t.Fatalf("dropped=%d", svc.stats.droppedTotal.Load())
	}
}

func TestFeedReaderEnqueuesAndReports(t *testing.T) {
	svc := &Service{
		cfg:    Config{FlushInterval: 50 * time.Millisecond},
		lineCh: make(chan ingestedLine, 32),
		stats:  newStats(),
	}

	body := strings.NewReader("not-a-valid-firewall-line\nanother-junk\n")
	stats, err := svc.FeedReader(context.Background(), body, "http")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Received != 2 {
		t.Fatalf("received=%d want 2", stats.Received)
	}
	if stats.Queued != 2 {
		t.Fatalf("queued=%d want 2", stats.Queued)
	}
	if stats.Dropped != 0 {
		t.Fatalf("dropped=%d", stats.Dropped)
	}
	// Async processing counters are not request-scoped.
	if stats.Parsed != 0 || stats.Inserted != 0 {
		t.Fatalf("parsed/inserted should be 0 (async), got parsed=%d inserted=%d", stats.Parsed, stats.Inserted)
	}
	if len(svc.lineCh) != 2 {
		t.Fatalf("queue depth=%d want 2", len(svc.lineCh))
	}
}

func TestFeedReaderDropsWhenQueueFull(t *testing.T) {
	svc := &Service{
		cfg:    Config{FlushInterval: 20 * time.Millisecond},
		lineCh: make(chan ingestedLine, 1),
		stats:  newStats(),
	}
	// Не читаем из очереди — первая строка займёт слот, остальные drop.
	body := strings.NewReader("line-1\nline-2\nline-3\n")
	stats, err := svc.FeedReader(context.Background(), body, "http")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Received != 3 {
		t.Fatalf("received=%d want 3", stats.Received)
	}
	if stats.Queued != 1 || stats.Dropped != 2 {
		t.Fatalf("queued=%d dropped=%d want 1/2", stats.Queued, stats.Dropped)
	}
}
