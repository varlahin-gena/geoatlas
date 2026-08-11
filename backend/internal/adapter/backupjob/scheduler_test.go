package backupjob

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type stubRunner struct {
	n atomic.Int32
}

func (s *stubRunner) TickAutoCreate(context.Context, time.Time) {
	s.n.Add(1)
}

func TestSchedulerTicksAndShutdown(t *testing.T) {
	r := &stubRunner{}
	s := New(r, 30*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	s.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutCancel()
	s.Shutdown(shutCtx)
	if r.n.Load() < 1 {
		t.Fatal("expected at least one tick")
	}
}
