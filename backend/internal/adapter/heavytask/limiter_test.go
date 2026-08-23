package heavytask

import (
	"context"
	"testing"
	"time"
)

func TestLimiterTryAcquireRelease(t *testing.T) {
	l := New(1)
	if !l.TryAcquire() {
		t.Fatal("first acquire")
	}
	if l.TryAcquire() {
		t.Fatal("second should fail")
	}
	if !l.Busy() {
		t.Fatal("expected busy")
	}
	l.Release()
	if l.Busy() {
		t.Fatal("expected idle")
	}
	if !l.TryAcquire() {
		t.Fatal("re-acquire")
	}
	l.Release()
}

func TestLimiterAcquireCancel(t *testing.T) {
	l := New(1)
	if !l.TryAcquire() {
		t.Fatal("hold slot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := l.Acquire(ctx); err == nil {
		t.Fatal("expected ctx error")
	}
	l.Release()
}

func TestLimiterNilSafe(t *testing.T) {
	var l *Limiter
	if !l.TryAcquire() {
		t.Fatal("nil try")
	}
	if err := l.Acquire(context.Background()); err != nil {
		t.Fatal(err)
	}
	l.Release()
	if l.Busy() {
		t.Fatal("nil busy")
	}
}

func TestDeferredGate(t *testing.T) {
	g := NewDeferredGate()
	if g.SkipReason() != "" {
		t.Fatalf("empty: %q", g.SkipReason())
	}
	g.Set(staticSkip("circuit"))
	if g.SkipReason() != "circuit" {
		t.Fatalf("got %q", g.SkipReason())
	}
	var nilGate *DeferredGate
	if nilGate.SkipReason() != "" {
		t.Fatal("nil gate")
	}
}

type staticSkip string

func (s staticSkip) SkipReason() string { return string(s) }
