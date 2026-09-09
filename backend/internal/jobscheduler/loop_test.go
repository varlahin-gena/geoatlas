package jobscheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoopTicksAndShutdown(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	loop := NewLoop()
	ctx, cancel := context.WithCancel(context.Background())
	loop.Start(ctx, 20*time.Millisecond, func(context.Context) {
		n.Add(1)
	})
	time.Sleep(70 * time.Millisecond)
	cancel()
	shut, shutCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutCancel()
	loop.Shutdown(shut)
	if n.Load() < 2 {
		t.Fatalf("ticks = %d, want >= 2", n.Load())
	}
	select {
	case <-loop.Done():
	default:
		t.Fatal("done not closed")
	}
}

func TestLoopCloseDone(t *testing.T) {
	t.Parallel()
	loop := NewLoop()
	loop.CloseDone()
	loop.CloseDone() // idempotent
	select {
	case <-loop.Done():
	default:
		t.Fatal("done not closed")
	}
	shut, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	loop.Shutdown(shut)
	if shut.Err() != nil {
		t.Fatal("Shutdown blocked after CloseDone")
	}
}

func TestLoopNilReceiver(t *testing.T) {
	t.Parallel()
	var loop *Loop
	loop.Start(context.Background(), time.Second, func(context.Context) {})
	loop.CloseDone()
	loop.Shutdown(context.Background())
	<-loop.Done()
}
